package kustomizepatch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/fluxcd/pkg/untar"
	applicationv1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/application/v1alpha1"
	"github.com/giantswarm/app/v4/pkg/app"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	vaultapi "github.com/hashicorp/vault/api"
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
)

const (
	nameSuffix          = "konfigure"
	giantswarmNamespace = "giantswarm"

	// cacheDir is a subfolder where konfigure will keep its cache if it's
	// running in cluster and talking to source-controller.
	cacheDir         = "cache"
	cacheLastModFile = "lastmod"
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
		return microerror.Mask(err)
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

		var vaultClient *vaultapi.Client
		{
			vaultClient, err = createVaultClientUsingEnv(ctx)
			if err != nil {
				return nil, microerror.Mask(err)
			}
		}

		// TODO(kuba):
		// If a `dir` is given in config we no longer care about sourceServiceEnvVar.
		// Else:
		// REQUIRES GitRepository pointing to giantswarm/config!
		// 1. Check if sourceServiceEnvVar is a valid URL (format: http://<kubernetes service ref>)
		// 1.5. (optional) Is it possible to check if we have the latest version pulled?
		// 2. Download http://<sourceServiceEnvVar>/gitrepository/<namespace>/<name>/latest.tar.gz
		//    example: http://<>/gitrepository/flux-system/gitrepository-giantswarm-config/latest.tar.gz
		//    if: there is a GitRepository CR 'flux-system/gitrepository-giantswarm-config'
		// 3. Untar with github.com/fluxcd/pkg/untar (untar.Untar())
		// 4. Set dir to location of untarred files
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

		var gen *generator.Service
		{
			c := generator.Config{
				VaultClient: vaultClient,

				Dir:          dir,
				Installation: installation,
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
				meta.Annotation.XCreator.Key():        meta.Annotation.XCreator.Default(),
				meta.Annotation.XInstallation.Key():   installation,
				meta.Annotation.XProjectVersion.Key(): meta.Annotation.XProjectVersion.Val(false),
			},
			ExtraLabels: nil,
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

func (r *runner) updateConfig() error {
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

	// Make a HEAD request. This allows us to check if the artifact we have
	// cached is still fresh - we will check the 'Last-Modified' header.
	client := &http.Client{Timeout: 15 * time.Second}
	request, err := http.NewRequest("HEAD", fmt.Sprintf("http://%s/gitrepository/%s/latest.tar.gz", svc, repo), nil)
	if err != nil {
		return microerror.Mask(err)
	}

	response, err := client.Do(request)
	if err != nil {
		return microerror.Mask(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return microerror.Maskf(
			executionFailedError,
			"error calling %q: expected %d, got %d", request.URL, http.StatusOK, response.StatusCode,
		)
	}

	// Figure out if the cache is still fresh. If no artifacts have been pulled
	// yet, download it for the first time.
	var cacheUpToDate = true // nolint: ineffassign

	sourceLastModified := response.Header.Get("Last-Modified")
	if sourceLastModified == "" {
		return microerror.Maskf(executionFailedError, "%s does not expose Last-Modified header", request.URL.String())
	}

	cacheLastModified, err := os.ReadFile(path.Join(cacheDir, cacheLastModFile))
	if err != nil && os.IsNotExist(err) {
		err = os.WriteFile(path.Join(cacheDir, cacheLastModFile), []byte(sourceLastModified), 0755) // nolint:gosec
		if err != nil {
			return microerror.Mask(err)
		}
		cacheUpToDate = false // file did not exist until now
	} else if err != nil {
		return microerror.Mask(err)
	} else {
		// Compare the time source-controller advertises as Last-Modified with
		// the time we saved last time an artifact was downloaded and cached.
		timeSourceLastModified, err := time.Parse(time.RFC1123, sourceLastModified)
		if err != nil {
			return microerror.Mask(err)
		}
		timeCacheLastModified, err := time.Parse(time.RFC1123, string(cacheLastModified))
		if err != nil {
			return microerror.Mask(err)
		}
		cacheUpToDate = timeCacheLastModified.Equal(timeSourceLastModified) || timeCacheLastModified.After(timeSourceLastModified)
	}

	if cacheUpToDate {
		return nil // early exit, cache matches the file served by source-controller
	}

	// Cache is stale, pull the latest artifact.
	request.Method = "GET" // reuse the request we used to ask for HEAD
	getResponse, err := client.Do(request)
	if err != nil {
		return microerror.Mask(err)
	}
	if getResponse.StatusCode != http.StatusOK {
		return microerror.Maskf(
			executionFailedError,
			"error calling %q: expected %d, got %d", request.URL, http.StatusOK, getResponse.StatusCode,
		)
	}
	defer getResponse.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, getResponse.Body)
	if err != nil {
		return microerror.Mask(err)
	}

	// Clear the old artifact's directory and untar a fresh one.
	dir := path.Join(cacheDir, "latest")
	if err := os.RemoveAll(dir); err != nil {
		return microerror.Mask(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return microerror.Mask(err)
	}
	if _, err = untar.Untar(&buf, dir); err != nil {
		return microerror.Mask(err)
	}

	// Update the timestamp
	err = os.WriteFile(path.Join(cacheDir, cacheLastModFile), []byte(sourceLastModified), 0755) // nolint:gosec
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
