package configupdater

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluxcd/pkg/tar"
	"github.com/giantswarm/microerror"
)

const (
	cacheLastArchive          = "lastarchive"
	cacheLastArchiveTimestamp = "lastarchivetimestamp"

	// v1SourceAPIGroup holds Flux Source group and v1 version
	v1SourceAPIGroup = "source.toolkit.fluxcd.io/v1"
	// v1beta2SourceAPIGroup holds Flux Source group and v1beta2 version
	v1beta2SourceAPIGroup = "source.toolkit.fluxcd.io/v1beta2"

	// defaultKubernetesTokenFile holds the location of the Kubernetes Service Account
	// token mount within a Pod.
	defaultKubernetesTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101
)

type Config struct {
	CacheDir string

	ApiServerHost       string
	ApiServerPort       string
	KubernetesTokenFile string

	SourceControllerService string

	GitRepository string
}

type FluxUpdater struct {
	CacheDir string

	ApiServerHost       string
	ApiServerPort       string
	KubernetesTokenFile string

	SourceControllerService string

	GitRepository string
}

func New(config Config) (*FluxUpdater, error) {
	if config.CacheDir == "" {
		return nil, microerror.Maskf(invalidConfigError, "cacheDir must not be empty")
	}

	if config.ApiServerHost == "" {
		return nil, microerror.Maskf(invalidConfigError, "apiServerHost must not be empty")
	}

	if config.ApiServerPort == "" {
		return nil, microerror.Maskf(invalidConfigError, "apiServerPort must not be empty")
	}

	var kubernetesTokenFile string
	if config.KubernetesTokenFile == "" {
		kubernetesTokenFile = defaultKubernetesTokenFile
	} else {
		kubernetesTokenFile = config.KubernetesTokenFile
	}

	if config.SourceControllerService == "" {
		return nil, microerror.Maskf(invalidConfigError, "sourceControllerService must not be empty")
	}

	if config.GitRepository == "" {
		return nil, microerror.Maskf(invalidConfigError, "gitRepository must not be empty")
	}

	if kubernetesTokenFile == "" {
		kubernetesTokenFile = defaultKubernetesTokenFile
	}

	return &FluxUpdater{
		CacheDir:                config.CacheDir,
		ApiServerHost:           config.ApiServerHost,
		ApiServerPort:           config.ApiServerPort,
		KubernetesTokenFile:     kubernetesTokenFile,
		SourceControllerService: config.SourceControllerService,
		GitRepository:           config.GitRepository,
	}, nil
}

// UpdateConfig makes sure that the assembled CCR version we keep stashed
// in <cacheDir>/latest is still *the* latest version out there. In order to do that,
// it sends a HEAD request for the last known artifact to the Source Controller,
// in order to check it is still available. If so, it then skips further processing.
// Otherwise, it contacts the GitRepository resource for the new artifact's URL.
// The URL is then used to download a new version of the archive and untar it.
// The archive name is being saved for later comparison.
func (u *FluxUpdater) UpdateConfig() error {
	// We first get the 'lastarchive' file, because it contains the name of the artifact we have
	// been using up until now. If the file is gone, it means we haven't populated the cache yet,
	// hence we must do it now. If the file is present, but archive of the given name is no longer
	// advertised by the Source Controller, we must look for a new one and re-populate the cache. If the
	// file is present, and is still advertised by the Source Controller, all is good and we may return.
	cachedArtifact, err := os.ReadFile(path.Join(u.CacheDir, cacheLastArchive))
	if err != nil && os.IsNotExist(err) {
		cachedArtifact = []byte("placeholder.tar.gz")
	} else if err != nil {
		return microerror.Mask(err)
	}

	cachedArtifactTimestampByte, err := os.ReadFile(path.Join(u.CacheDir, cacheLastArchiveTimestamp))
	if err != nil && os.IsNotExist(err) {
		cachedArtifactTimestampByte = []byte(time.Time{}.Format(http.TimeFormat))
	} else if err != nil {
		return microerror.Mask(err)
	}

	cachedArtifactTimestamp, err := time.Parse(http.TimeFormat, string(cachedArtifactTimestampByte))
	if err != nil {
		return microerror.Mask(err)
	}

	url := fmt.Sprintf("http://%s/gitrepository/%s/%s", u.SourceControllerService, u.GitRepository, string(cachedArtifact))
	// Make a HEAD request to the Source Controller. This allows us to check if the artifact
	// we have cached is still offered.
	client := &http.Client{Timeout: 60 * time.Second}
	request, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return microerror.Mask(err)
	}

	response, err := client.Do(request)
	if err != nil {
		return microerror.Mask(err)
	}
	defer response.Body.Close()

	// The artifact we were asking for is still advertised by the Source Controller,
	// and has not changed since the last time, hence we may skip further processing.
	if response.StatusCode == http.StatusOK {
		// The artifact we are asking for is still available, we need to check its
		// last modification date
		artifactTimestamp, err := time.Parse(http.TimeFormat, response.Header.Get("Last-Modified"))
		if err != nil {
			return microerror.Mask(err)
		}

		if cachedArtifactTimestamp.After(artifactTimestamp) || cachedArtifactTimestamp.Equal(artifactTimestamp) {
			return nil
		}
	} else {
		if response.StatusCode != http.StatusNotFound {
			return microerror.Maskf(
				executionFailedError,
				"error calling %q: expected %d, got %d", request.URL, http.StatusNotFound, response.StatusCode,
			)
		} else {
			url = ""
		}
	}

	// When latest known revision is still available, there is no need to query the API Server
	// for the GitRepository, it saves us one call.
	if url == "" {
		repoCoordinates := strings.Split(u.GitRepository, "/")

		k8sApiPath := []string{
			fmt.Sprintf(
				"https://%s:%s/apis/%s/namespaces/%s/gitrepositories/%s",
				u.ApiServerHost,
				u.ApiServerPort,
				v1SourceAPIGroup,
				repoCoordinates[0],
				repoCoordinates[1],
			),
			fmt.Sprintf(
				"https://%s:%s/apis/%s/namespaces/%s/gitrepositories/%s",
				u.ApiServerHost,
				u.ApiServerPort,
				v1beta2SourceAPIGroup,
				repoCoordinates[0],
				repoCoordinates[1],
			),
		}

		k8sToken, err := os.ReadFile(u.KubernetesTokenFile)
		if err != nil {
			return microerror.Mask(err)
		}

		bearer := fmt.Sprintf("Bearer %s", strings.TrimSpace(string(k8sToken)))

		// Make a GET request to the Kubernetes API server to get the GitRepository
		// in a JSON format.
		for _, p := range k8sApiPath {
			request, err = http.NewRequest(http.MethodGet, p, nil)
			if err != nil {
				return microerror.Mask(err)
			}

			request.Header.Set("Authorization", bearer)
			request.Header.Add("Accept", "application/json")

			tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // nolint:gosec
			client.Transport = tr

			response, err = client.Do(request)
			if err != nil {
				return microerror.Mask(err)
			}
			defer response.Body.Close()

			if response.StatusCode == http.StatusOK {
				break
			}

			if response.StatusCode == http.StatusNotFound {
				continue
			}

			return microerror.Maskf(
				executionFailedError,
				"error calling %q: expected %d, got %d", request.URL, http.StatusOK, response.StatusCode,
			)
		}

		if response.StatusCode != http.StatusOK {
			return microerror.Maskf(
				executionFailedError,
				"error getting '%s' GitRepository CR", u.GitRepository,
			)
		}

		responseBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return microerror.Mask(err)
		}

		// We are not interested in an entire object, we are only interested in getting
		// some of the status fields that advertise the new archive.
		type gitRepository struct {
			Status struct {
				Artifact struct {
					Url string
				}
			}
		}

		var gr gitRepository
		err = json.Unmarshal(responseBytes, &gr)
		if err != nil {
			return microerror.Mask(err)
		}

		// Note: technically this does not mean an error. An empty field could be a symptom
		// of the CR still being reconciled, or not being picked up by the Source Controller
		// at all, in which case, we could simply skip quietly.
		if gr.Status.Artifact.Url == "" {
			return microerror.Maskf(
				executionFailedError,
				"error downloading artifact: got empty URL from GitRepository status",
			)
		}

		url = gr.Status.Artifact.Url
	}

	request, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return microerror.Mask(err)
	}

	response, err = client.Do(request)
	if err != nil {
		return microerror.Mask(err)
	}
	if response.StatusCode != http.StatusOK {
		return microerror.Maskf(
			executionFailedError,
			"error calling %q: expected %d, got %d", request.URL, http.StatusOK, response.StatusCode,
		)
	}
	defer response.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, response.Body)
	if err != nil {
		return microerror.Mask(err)
	}

	// Clear the old artifact's directory and untar a fresh one.
	dir := path.Join(u.CacheDir, "latest")
	if err := os.RemoveAll(dir); err != nil {
		return microerror.Mask(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return microerror.Mask(err)
	}
	if err = tar.Untar(&buf, dir); err != nil {
		return microerror.Mask(err)
	}

	// Update the last archive name and timestamp
	err = os.WriteFile(path.Join(u.CacheDir, cacheLastArchive), []byte(filepath.Base(url)), 0755) // nolint:gosec
	if err != nil {
		return microerror.Mask(err)
	}

	err = os.WriteFile(path.Join(u.CacheDir, cacheLastArchiveTimestamp), []byte(response.Header.Get("Last-Modified")), 0755) // nolint:gosec
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
