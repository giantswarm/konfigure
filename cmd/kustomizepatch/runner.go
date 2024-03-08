package kustomizepatch

import (
	"bytes"
	"context"
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

	"github.com/fluxcd/pkg/untar"
	applicationv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/app/v7/pkg/app"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/konfigure/internal/generator"
	"github.com/giantswarm/konfigure/internal/meta"
	"github.com/giantswarm/konfigure/internal/sopsenv/key"
	"github.com/giantswarm/konfigure/internal/vaultclient"
)

const (
	nameSuffix          = "konfigure"
	giantswarmNamespace = "giantswarm"

	// cacheDir is a directory where konfigure will keep its cache if it's
	// running in cluster and talking to source-controller.
	cacheDir                  = "/tmp/konfigure-cache"
	cacheLastArchive          = "lastarchive"
	cacheLastArchiveTimestamp = "lastarchivetimestamp"
	// dirEnvVar is a directory containing giantswarm/config. If set, requests
	// to source-controller will not be made and both sourceServiceEnvVar and
	// gitRepositoryEnvVar will be ignored. Used only on local machine for
	// debugging.
	dirEnvVar = "KONFIGURE_DIR"
	// installationEnvVar tells konfigure which installation it's running in,
	// e.g. "ginger"
	installationEnvVar = "KONFIGURE_INSTALLATION"
	// gitRepositoryEnvVar is namespace/name of GitRepository pointing to
	// giantswarm/config, e.g. "flux-system/gs-config"
	gitRepositoryEnvVar = "KONFIGURE_GITREPO"
	// kubernetesServiceEnvVar is K8S host of the Kubernetes API service.
	kubernetesServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"
	// kubernetesServicePortEnvVar is K8S port of the Kubernetes API service.
	kubernetesServicePortEnvVar = "KUBERNETES_SERVICE_PORT"
	// kubernetesToken holds the location of the Kubernetes Service Account
	// token mount within a Pod.
	kubernetesTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101
	// sopsKeysDirEnvVar tells Konfigure how to configure environment to make
	// it possible for SOPS to find the keys
	sopsKeysDirEnvVar = "KONFIGURE_SOPS_KEYS_DIR"
	// sopsKeysSourceEnvVar tells Konfigure to either get keys from Kubernetes
	// Secrets or rely on local storage when setting up environment for SOPS
	sopsKeysSourceEnvVar = "KONFIGURE_SOPS_KEYS_SOURCE"
	// v1SourceAPIGroup holds Flux Source group and v1 version
	v1SourceAPIGroup = "source.toolkit.fluxcd.io/v1"
	// v1beta2SourceAPIGroup holds Flux Source group and v1beta2 version
	v1beta2SourceAPIGroup = "source.toolkit.fluxcd.io/v1beta2"
	// sourceServiceEnvVar is K8s address of source-controller's service, e.g.
	// "source-controller.flux-system.svc"
	sourceServiceEnvVar = "KONFIGURE_SOURCE_SERVICE"
)

type runner struct {
	logger micrologger.Logger
	stdout io.Writer
	stderr io.Writer

	config *config
}

func (r *runner) Run(_ *cobra.Command, _ []string) error {
	r.config = new(config)

	processor := framework.SimpleProcessor{
		Config: r.config,
		Filter: kio.FilterFunc(r.run),
	}
	pluginCmd := command.Build(processor, command.StandaloneDisabled, false)

	err := pluginCmd.Execute()
	if err != nil {
		// print pretty error for the sake of kustomize-controller logs
		r.logger.Errorf(context.Background(), err, "konfigure encountered an error")
		return fmt.Errorf("error %w\noccurred with konfigure input: %+v", err, r.config)
	}

	return nil
}

func (r *runner) run(items []*kyaml.RNode) ([]*kyaml.RNode, error) {
	ctx := context.Background()

	var configmap *corev1.ConfigMap
	var secret *corev1.Secret
	var err error
	{
		if r.config == nil {
			return nil, microerror.Maskf(invalidConfigError, "r.config is required, got <nil>")
		}

		if err := r.config.Validate(); err != nil {
			return nil, microerror.Mask(err)
		}

		var installation string
		{
			installation = os.Getenv(installationEnvVar)
			if installation == "" {
				return nil, microerror.Maskf(invalidConfigError, "%q environment variable is required", installationEnvVar)
			}
		}

		var vaultClient *vaultclient.WrappedVaultClient
		{
			vaultClient, err = vaultclient.NewClientUsingEnv(ctx)
			if err != nil {
				return nil, microerror.Mask(err)
			}
		}

		var dir string
		{
			// If the dirEnvVar is set we don't communicate with
			// source-controller. Use what's in the dir.
			dir = os.Getenv(dirEnvVar)
			// Else, we download the packaged config from source-controller.
			if dir == "" {
				if err := r.updateConfig(); err != nil {
					return nil, microerror.Mask(err)
				}
				dir = path.Join(cacheDir, "latest")
			}
		}

		var sopsKeysSource string
		{
			sopsKeysSource = os.Getenv(sopsKeysSourceEnvVar)

			if sopsKeysSource == "" {
				sopsKeysSource = key.KeysSourceLocal
			}

			if sopsKeysSource != key.KeysSourceLocal && sopsKeysSource != key.KeysSourceKubernetes {
				return nil, microerror.Maskf(invalidConfigError, "%q environment variable wrong value, must be one of: local,kubernetes\n", sopsKeysSourceEnvVar)
			}
		}

		var gen *generator.Service
		{
			c := generator.Config{
				VaultClient: vaultClient,

				Log:            r.logger,
				Dir:            dir,
				Installation:   installation,
				SOPSKeysDir:    os.Getenv(sopsKeysDirEnvVar),
				SOPSKeysSource: sopsKeysSource,
			}

			gen, err = generator.New(c)
			if err != nil {
				return nil, microerror.Mask(err)
			}
		}

		in := generator.GenerateInput{
			App:       r.config.AppName,
			Name:      addNameSuffix(r.config.Name),
			Namespace: giantswarmNamespace,

			ExtraAnnotations: map[string]string{
				meta.Annotation.XAppInfo.Key():        meta.Annotation.XAppInfo.Val(r.config.AppCatalog, r.config.AppName, r.config.AppVersion),
				meta.Annotation.XCreator.Key():        "konfigure",
				meta.Annotation.XInstallation.Key():   installation,
				meta.Annotation.XProjectVersion.Key(): meta.Annotation.XProjectVersion.Val(false),
			},
			ExtraLabels:     nil,
			VersionOverride: "main",
		}

		configmap, secret, err = gen.Generate(ctx, in)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var appCR *applicationv1alpha1.App
	{
		c := app.Config{
			AppCatalog:          r.config.AppCatalog,
			AppName:             r.config.AppName,
			AppNamespace:        r.config.AppDestinationNamespace,
			AppVersion:          r.config.AppVersion,
			ConfigVersion:       configmap.Annotations[meta.Annotation.ConfigVersion.Key()],
			DisableForceUpgrade: r.config.AppDisableForceUpgrade,
			Name:                r.config.Name,
			InCluster:           true,
			Labels: map[string]string{
				meta.Label.ManagedBy.Key(): meta.Label.ManagedBy.Default(),
			},
		}

		appCR = app.NewCR(c)

		appCR.Spec.Config.ConfigMap = applicationv1alpha1.AppSpecConfigConfigMap{
			Name:      configmap.Name,
			Namespace: configmap.Namespace,
		}
		appCR.Spec.Config.Secret = applicationv1alpha1.AppSpecConfigSecret{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		}
	}

	output := []*kyaml.RNode{}
	for _, item := range []runtime.Object{configmap, secret, appCR} {
		b, err := yaml.Marshal(item)
		if err != nil {
			return nil, microerror.Maskf(
				executionFailedError,
				"error marshalling %s/%s %s: %s",
				item.GetObjectKind().GroupVersionKind().Group,
				item.GetObjectKind().GroupVersionKind().Version,
				item.GetObjectKind().GroupVersionKind().Kind,
				err,
			)
		}

		rnode, err := kyaml.Parse(string(b))
		if err != nil {
			return nil, microerror.Maskf(
				executionFailedError,
				"error parsing %s/%s %s: %s",
				item.GetObjectKind().GroupVersionKind().Group,
				item.GetObjectKind().GroupVersionKind().Version,
				item.GetObjectKind().GroupVersionKind().Kind,
				err,
			)
		}

		output = append(output, rnode)
	}

	return output, nil
}

// updateConfig makes sure that the giantswarm/config version we keep stashed
// in <cacheDir>/latest is still *the* latest version out there. In order to do that,
// it sends a HEAD request for the last known artifact to the Source Controller,
// in order to check it is still available. If so, it then skips further processing.
// Otherwise it contacts the GitRepository resource for the new artifact's URL.
// The URL is then used to download a new version of the archive and untar it.
// The archive name is being saved for later comparison.
func (r *runner) updateConfig() error {
	return r.updateConfigWithParams(cacheDir, kubernetesTokenFile)
}

func (r *runner) updateConfigWithParams(cache, token string) error {
	// Get source-controller's service URL and GitRepository data from
	// environment variables. We use this data to construct an URL to
	// source-controller's artifact.
	svc := os.Getenv(sourceServiceEnvVar)
	if svc == "" {
		return microerror.Maskf(executionFailedError, "%q environment variable not set", sourceServiceEnvVar)
	}

	repo := os.Getenv(gitRepositoryEnvVar)
	if repo == "" {
		return microerror.Maskf(executionFailedError, "%q environment variable not set", gitRepositoryEnvVar)
	}

	// We first get the 'lastarchive' file, because it contains the name of the artifact we have
	// been using up until now. If the file is gone, it means we haven't populated the cache yet,
	// hence we must do it now. If the file is present, but archive of the given name is no longer
	// advertised by the Source Controller, we must look for a new one and re-populate the cache. If the
	// file is present, and is still advertised by the Source Controller, all is good and we may return.
	cachedArtifact, err := os.ReadFile(path.Join(cache, cacheLastArchive))
	if err != nil && os.IsNotExist(err) {
		cachedArtifact = []byte("placeholder.tar.gz")
	} else if err != nil {
		return microerror.Mask(err)
	}

	cachedArtifactTimestampByte, err := os.ReadFile(path.Join(cache, cacheLastArchiveTimestamp))
	if err != nil && os.IsNotExist(err) {
		cachedArtifactTimestampByte = []byte(time.Time{}.Format(http.TimeFormat))
	} else if err != nil {
		return microerror.Mask(err)
	}

	cachedArtifactTimestamp, err := time.Parse(http.TimeFormat, string(cachedArtifactTimestampByte))
	if err != nil {
		return microerror.Mask(err)
	}

	url := fmt.Sprintf("http://%s/gitrepository/%s/%s", svc, repo, string(cachedArtifact))
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
		// The artifact we were asking for is gone, we must find the newly advertised one,
		// hence we query the Kubernetes API Server for the GitRepository CR resource.
		k8sApiHost := os.Getenv(kubernetesServiceHostEnvVar)
		if svc == "" {
			return microerror.Maskf(executionFailedError, "%q environment variable not set", kubernetesServiceHostEnvVar)
		}

		k8sApiPort := os.Getenv(kubernetesServicePortEnvVar)
		if svc == "" {
			return microerror.Maskf(executionFailedError, "%q environment variable not set", kubernetesServicePortEnvVar)
		}

		repoCoordinates := strings.Split(repo, "/")

		k8sApiPath := []string{
			fmt.Sprintf(
				"https://%s:%s/apis/%s/namespaces/%s/gitrepositories/%s",
				k8sApiHost,
				k8sApiPort,
				v1SourceAPIGroup,
				repoCoordinates[0],
				repoCoordinates[1],
			),
			fmt.Sprintf(
				"https://%s:%s/apis/%s/namespaces/%s/gitrepositories/%s",
				k8sApiHost,
				k8sApiPort,
				v1beta2SourceAPIGroup,
				repoCoordinates[0],
				repoCoordinates[1],
			),
		}

		k8sToken, err := os.ReadFile(token)
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
				"error getting '%s' GitRepository CR", repo,
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
	dir := path.Join(cache, "latest")
	if err := os.RemoveAll(dir); err != nil {
		return microerror.Mask(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return microerror.Mask(err)
	}
	if _, err = untar.Untar(&buf, dir); err != nil {
		return microerror.Mask(err)
	}

	// Update the last archive name and timestamp
	err = os.WriteFile(path.Join(cache, cacheLastArchive), []byte(filepath.Base(url)), 0755) // nolint:gosec
	if err != nil {
		return microerror.Mask(err)
	}

	err = os.WriteFile(path.Join(cache, cacheLastArchiveTimestamp), []byte(response.Header.Get("Last-Modified")), 0755) // nolint:gosec
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func addNameSuffix(name string) string {
	if len(name) >= 63-len(nameSuffix)-1 {
		name = name[:63-len(nameSuffix)-1]
	}
	name = strings.TrimSuffix(name, "-")
	return fmt.Sprintf("%s-%s", name, nameSuffix)
}
