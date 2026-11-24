package fluxupdater

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/fluxcd/pkg/tar"
)

const (
	cacheLastArchive          = "lastarchive"
	cacheLastArchiveTimestamp = "lastarchivetimestamp"
	cacheLastArtifactUrl      = "lastartifacturl"

	// v1SourceAPIGroup holds Flux Source group and v1 version
	v1SourceAPIGroup = "source.toolkit.fluxcd.io/v1"
	// v1beta2SourceAPIGroup holds Flux Source group and v1beta2 version
	v1beta2SourceAPIGroup = "source.toolkit.fluxcd.io/v1beta2"

	// defaultKubernetesTokenFile holds the location of the Kubernetes Service Account
	// token mount within a Pod.
	defaultKubernetesTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101
)

const (
	// DefaultSourceControllerNamespace is the namespace of the Source
	// Controller that DefaultArtifactHosts trusts.
	DefaultSourceControllerNamespace = "flux-giantswarm"
	// DefaultClusterDomain is the cluster domain that DefaultArtifactHosts
	// uses.
	DefaultClusterDomain = "cluster.local"
)

// maxArtifactSize caps the downloaded artifact before it is extracted. It
// matches the untar limit for the extracted files. A variable so tests can
// lower it.
var maxArtifactSize int64 = tar.DefaultMaxUntarSize

// ArtifactHost returns the service host name of a Source Controller in
// namespace on a cluster with clusterDomain, for example
// "source-controller.flux-giantswarm.svc.cluster.local". An empty
// clusterDomain means DefaultClusterDomain. The allowed hosts must match the
// host the Source Controller advertises with its --storage-adv-addr flag.
func ArtifactHost(namespace, clusterDomain string) string {
	clusterDomain = strings.Trim(strings.TrimSpace(clusterDomain), ".")
	if clusterDomain == "" {
		clusterDomain = DefaultClusterDomain
	}
	return fmt.Sprintf("source-controller.%s.svc.%s", strings.TrimSpace(namespace), clusterDomain)
}

// DefaultArtifactHosts returns the hosts artifacts are downloaded from when
// Config.ArtifactHosts is empty: the Source Controller service in the
// flux-giantswarm namespace on a cluster with clusterDomain, or with
// DefaultClusterDomain when clusterDomain is empty.
func DefaultArtifactHosts(clusterDomain string) []string {
	return []string{ArtifactHost(DefaultSourceControllerNamespace, clusterDomain)}
}

type Config struct {
	CacheDir string

	ApiServerHost       string
	ApiServerPort       string
	KubernetesTokenFile string

	// GitRepository is the GitRepository to fetch, as "<namespace>/<name>".
	GitRepository string

	// ArtifactHosts lists the host names artifact URLs may point at. An
	// artifact URL advertised by the GitRepository, or cached from an
	// earlier update, is only used when its host is in this list and its
	// path is below /gitrepository/<namespace>/<name>/. Entries are host
	// names only, without scheme or port; the port is not checked. Defaults
	// to DefaultArtifactHosts(ClusterDomain) when empty. Use ArtifactHost
	// to build the entry for another namespace or cluster domain.
	ArtifactHosts []string

	// ClusterDomain is the cluster domain used for the default artifact
	// host when ArtifactHosts is empty, and ignored otherwise. Defaults to
	// DefaultClusterDomain.
	ClusterDomain string
}

type FluxUpdater struct {
	CacheDir string

	ApiServerHost       string
	ApiServerPort       string
	KubernetesTokenFile string

	GitRepository string

	ArtifactHosts []string

	gitRepositoryNamespace string
	gitRepositoryName      string
}

func New(config Config) (*FluxUpdater, error) {
	if config.CacheDir == "" {
		return nil, &InvalidConfigError{message: "cacheDir must not be empty"}
	}

	if config.ApiServerHost == "" {
		return nil, &InvalidConfigError{message: "apiServerHost must not be empty"}
	}

	if config.ApiServerPort == "" {
		return nil, &InvalidConfigError{message: "apiServerPort must not be empty"}
	}

	var kubernetesTokenFile string
	if config.KubernetesTokenFile == "" {
		kubernetesTokenFile = defaultKubernetesTokenFile
	} else {
		kubernetesTokenFile = config.KubernetesTokenFile
	}

	if config.GitRepository == "" {
		return nil, &InvalidConfigError{message: "gitRepository must not be empty"}
	}

	repoCoordinates := strings.Split(config.GitRepository, "/")
	if len(repoCoordinates) != 2 ||
		len(validation.IsDNS1123Label(repoCoordinates[0])) != 0 ||
		len(validation.IsDNS1123Subdomain(repoCoordinates[1])) != 0 {
		return nil, &InvalidConfigError{message: fmt.Sprintf("gitRepository must be <namespace>/<name> with valid Kubernetes names, got %q", config.GitRepository)}
	}

	clusterDomain := strings.Trim(strings.TrimSpace(config.ClusterDomain), ".")
	if clusterDomain != "" {
		if errs := validation.IsDNS1123Subdomain(clusterDomain); len(errs) != 0 {
			return nil, &InvalidConfigError{message: fmt.Sprintf("clusterDomain %q is not a valid domain: %s", config.ClusterDomain, strings.Join(errs, ", "))}
		}
	}

	configuredHosts := config.ArtifactHosts
	if len(configuredHosts) == 0 {
		configuredHosts = DefaultArtifactHosts(clusterDomain)
	}

	artifactHosts := make([]string, 0, len(configuredHosts))
	for _, host := range configuredHosts {
		host = normalizeHost(host)
		if host == "" {
			return nil, &InvalidConfigError{message: "artifactHosts must not contain empty host names"}
		}
		if strings.ContainsAny(host, ":/@?#[]") {
			return nil, &InvalidConfigError{message: fmt.Sprintf("artifactHosts entry %q must be a host name without scheme, port or path", host)}
		}
		if errs := validation.IsDNS1123Subdomain(host); len(errs) != 0 {
			return nil, &InvalidConfigError{message: fmt.Sprintf("artifactHosts entry %q is not a valid host name: %s", host, strings.Join(errs, ", "))}
		}
		// A host without the cluster domain is completed through the
		// resolver search list, which can resolve it to another record.
		if !strings.Contains(host, ".") || strings.HasSuffix(host, ".svc") {
			return nil, &InvalidConfigError{message: fmt.Sprintf("artifactHosts entry %q must be a fully qualified host name, for example %q", host, ArtifactHost(DefaultSourceControllerNamespace, DefaultClusterDomain))}
		}
		artifactHosts = append(artifactHosts, host)
	}

	if kubernetesTokenFile == "" {
		kubernetesTokenFile = defaultKubernetesTokenFile
	}

	return &FluxUpdater{
		CacheDir:            config.CacheDir,
		ApiServerHost:       config.ApiServerHost,
		ApiServerPort:       config.ApiServerPort,
		KubernetesTokenFile: kubernetesTokenFile,
		GitRepository:       config.GitRepository,

		ArtifactHosts: artifactHosts,

		gitRepositoryNamespace: repoCoordinates[0],
		gitRepositoryName:      repoCoordinates[1],
	}, nil
}

// normalizeHost lower-cases a host name and drops a trailing dot, so
// "source-controller.flux-giantswarm.svc.cluster.local." and
// "source-controller.flux-giantswarm.svc.cluster.local" compare equal.
func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

// validateArtifactURL checks that rawURL points at an allowed Source
// Controller host and at an artifact of the configured GitRepository.
func (u *FluxUpdater) validateArtifactURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return &ExecutionFailedError{message: fmt.Sprintf("invalid artifact URL: %s", err)}
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &ExecutionFailedError{message: fmt.Sprintf("artifact URL scheme %q is not allowed", parsed.Scheme)}
	}

	if parsed.Opaque != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return &ExecutionFailedError{message: "artifact URL must not contain user info, query or fragment"}
	}

	if !slices.Contains(u.ArtifactHosts, normalizeHost(parsed.Hostname())) {
		return &ExecutionFailedError{message: fmt.Sprintf("artifact URL host %q is not an allowed Source Controller host", parsed.Hostname())}
	}

	expectedPrefix := fmt.Sprintf("/gitrepository/%s/%s/", u.gitRepositoryNamespace, u.gitRepositoryName)
	if parsed.RawPath != "" || path.Clean(parsed.Path) != parsed.Path || !strings.HasPrefix(parsed.Path, expectedPrefix) || len(parsed.Path) == len(expectedPrefix) {
		return &ExecutionFailedError{message: fmt.Sprintf("artifact URL path %q is not below %q", parsed.Path, expectedPrefix)}
	}

	return nil
}

// UpdateConfig makes sure that the assembled CCR version we keep stashed
// in <cacheDir>/latest is still *the* latest version out there. In order to do that,
// it sends a HEAD request for the last known artifact to the Source Controller,
// in order to check it is still available. If so, it then skips further processing.
// Otherwise, it contacts the GitRepository resource for the new artifact's URL.
// The URL is then used to download a new version of the archive and untar it.
// The archive name is being saved for later comparison.
func (u *FluxUpdater) UpdateConfig() error {
	// We first get the 'lastarchivetimestamp' and 'lastartifacturl' files, because it contains the URL
	//of the artifact we have been using up until now. If the file is gone, it means we haven't populated the cache yet,
	// hence we must do it now. If the file is present, but archive of the given name is no longer
	// advertised by the Source Controller, we must look for a new one and re-populate the cache. If the
	// file is present, and is still advertised by the Source Controller, all is good and we may return.
	cachedArtifactTimestampByte, err := os.ReadFile(path.Join(u.CacheDir, cacheLastArchiveTimestamp))
	if err != nil && os.IsNotExist(err) {
		cachedArtifactTimestampByte = []byte(time.Time{}.Format(http.TimeFormat))
	} else if err != nil {
		return err
	}

	cachedArtifactTimestamp, err := time.Parse(http.TimeFormat, string(cachedArtifactTimestampByte))
	if err != nil {
		return err
	}

	cacheLastArtifactUrlByte, err := os.ReadFile(path.Join(u.CacheDir, cacheLastArtifactUrl))
	if err != nil && os.IsNotExist(err) {
		cacheLastArtifactUrlByte = []byte("")
	} else if err != nil {
		return err
	}

	artifactURL := string(cacheLastArtifactUrlByte)

	// A cached URL is only trusted while it still passes the same checks as a
	// freshly advertised one. Otherwise it is dropped and the GitRepository is
	// asked for the current artifact.
	if artifactURL != "" && u.validateArtifactURL(artifactURL) != nil {
		artifactURL = ""
	}

	// Artifacts are downloaded without following redirects, so a validated URL
	// cannot hand the request over to another host.
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	if artifactURL != "" {
		// Make a HEAD request to the Source Controller. This allows us to check if the artifact
		// we have cached is still offered.
		request, err := http.NewRequest(http.MethodHead, artifactURL, nil)
		if err != nil {
			return err
		}

		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer func() { _ = response.Body.Close() }()

		// The artifact we were asking for is still advertised by the Source Controller,
		// and has not changed since the last time, hence we may skip further processing.
		if response.StatusCode == http.StatusOK {
			// The artifact we are asking for is still available, we need to check its
			// last modification date
			artifactTimestamp, err := time.Parse(http.TimeFormat, response.Header.Get("Last-Modified"))
			if err != nil {
				return err
			}

			if cachedArtifactTimestamp.After(artifactTimestamp) || cachedArtifactTimestamp.Equal(artifactTimestamp) {
				return nil
			}
		} else {
			// The cached artifact is gone or cannot be checked, so ask the
			// GitRepository for the current one.
			artifactURL = ""
		}
	}

	// When latest known revision is still available, there is no need to query the API Server
	// for the GitRepository, it saves us one call.
	if artifactURL == "" {
		k8sApiPath := []string{
			fmt.Sprintf(
				"https://%s:%s/apis/%s/namespaces/%s/gitrepositories/%s",
				u.ApiServerHost,
				u.ApiServerPort,
				v1SourceAPIGroup,
				u.gitRepositoryNamespace,
				u.gitRepositoryName,
			),
			fmt.Sprintf(
				"https://%s:%s/apis/%s/namespaces/%s/gitrepositories/%s",
				u.ApiServerHost,
				u.ApiServerPort,
				v1beta2SourceAPIGroup,
				u.gitRepositoryNamespace,
				u.gitRepositoryName,
			),
		}

		k8sToken, err := os.ReadFile(u.KubernetesTokenFile)
		if err != nil {
			return err
		}

		bearer := fmt.Sprintf("Bearer %s", strings.TrimSpace(string(k8sToken)))

		// Make a GET request to the Kubernetes API server to get the GitRepository
		// in a JSON format.
		apiTransport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // nolint:gosec
		defer apiTransport.CloseIdleConnections()

		apiClient := &http.Client{
			Timeout:   client.Timeout,
			Transport: apiTransport,
		}

		var responseBytes []byte
		for _, p := range k8sApiPath {
			request, err := http.NewRequest(http.MethodGet, p, nil)
			if err != nil {
				return err
			}

			request.Header.Set("Authorization", bearer)
			request.Header.Add("Accept", "application/json")

			response, err := apiClient.Do(request)
			if err != nil {
				return err
			}

			statusCode := response.StatusCode
			if statusCode == http.StatusOK {
				responseBytes, err = io.ReadAll(response.Body)
			}
			_ = response.Body.Close()

			if statusCode == http.StatusOK {
				if err != nil {
					return err
				}
				break
			}

			if statusCode == http.StatusNotFound {
				continue
			}

			return &ExecutionFailedError{
				message: fmt.Sprintf("error calling %q: expected %d, got %d", request.URL, http.StatusOK, statusCode),
			}
		}

		if responseBytes == nil {
			return &ExecutionFailedError{
				message: fmt.Sprintf("error getting '%s' GitRepository CR", u.GitRepository),
			}
		}

		// We are not interested in an entire object, we are only interested in getting
		// some of the status fields that advertise the new archive.
		type gitRepository struct {
			Status struct {
				Conditions []metav1.Condition `json:"conditions,omitempty"`
				Artifact   struct {
					Url string
				}
			}
		}

		var gr gitRepository
		err = json.Unmarshal(responseBytes, &gr)
		if err != nil {
			return err
		}

		// Note: technically this does not mean an error. An empty field could be a symptom
		// of the CR still being reconciled, or not being picked up by the Source Controller
		// at all, in which case, we could simply skip quietly.
		if gr.Status.Artifact.Url == "" {
			for _, condition := range gr.Status.Conditions {
				if condition.Type == "Ready" && condition.Status != metav1.ConditionTrue {
					return &ExecutionFailedError{
						message: condition.Message,
					}
				}
			}

			return &ExecutionFailedError{
				message: "error downloading artifact: got empty URL from GitRepository status",
			}
		}

		if err := u.validateArtifactURL(gr.Status.Artifact.Url); err != nil {
			return err
		}

		artifactURL = gr.Status.Artifact.Url
	}

	request, err := http.NewRequest(http.MethodGet, artifactURL, nil)
	if err != nil {
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return &ExecutionFailedError{
			message: fmt.Sprintf("error calling %q: expected %d, got %d", request.URL, http.StatusOK, response.StatusCode),
		}
	}

	if response.ContentLength > maxArtifactSize {
		return &ExecutionFailedError{
			message: fmt.Sprintf("error downloading %q: artifact is larger than %d bytes", request.URL, maxArtifactSize),
		}
	}

	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(response.Body, maxArtifactSize+1))
	if err != nil {
		return err
	}
	if n > maxArtifactSize {
		return &ExecutionFailedError{
			message: fmt.Sprintf("error downloading %q: artifact is larger than %d bytes", request.URL, maxArtifactSize),
		}
	}

	// Forget the cached artifact before touching the config directory. If
	// extraction fails or the process stops half-way, the next update then
	// downloads the artifact again instead of keeping a partial config. The
	// config is extracted in place, so the cache never needs room for two
	// copies of it.
	for _, name := range []string{cacheLastArtifactUrl, cacheLastArchiveTimestamp, cacheLastArchive} {
		if err := os.Remove(path.Join(u.CacheDir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Clear the old artifact's directory and untar a fresh one.
	dir := path.Join(u.CacheDir, "latest")
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	if err = tar.Untar(&buf, dir); err != nil {
		return err
	}

	// Update the last archive name, timestamp and url
	err = os.WriteFile(path.Join(u.CacheDir, cacheLastArchive), []byte(filepath.Base(artifactURL)), 0750) // nolint:gosec
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(u.CacheDir, cacheLastArchiveTimestamp), []byte(response.Header.Get("Last-Modified")), 0750) // nolint:gosec
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(u.CacheDir, cacheLastArtifactUrl), []byte(artifactURL), 0750) // nolint:gosec
	if err != nil {
		return err
	}

	return nil
}
