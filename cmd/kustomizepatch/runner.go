package kustomizepatch

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

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

	"github.com/giantswarm/konfigure/pkg/fluxupdater"
	"github.com/giantswarm/konfigure/pkg/meta"
	"github.com/giantswarm/konfigure/pkg/service"
	"github.com/giantswarm/konfigure/pkg/sopsenv/key"
	"github.com/giantswarm/konfigure/pkg/vaultclient"
)

const (
	nameSuffix          = "konfigure"
	giantswarmNamespace = "giantswarm"

	// cacheDir is a directory where konfigure will keep its cache if it's
	// running in cluster and talking to source-controller.
	cacheDir = "/tmp/konfigure-cache"

	// dirEnvVar is a directory containing giantswarm/config. If set, requests
	// to source-controller will not be made and both sourceServiceEnvVar and
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

	// sourceServiceEnvVar is K8s address of source-controller's service, e.g.
	// "source-controller.flux-system.svc"
	sourceServiceEnvVar = "KONFIGURE_SOURCE_SERVICE"

	// sopsKeysDirEnvVar tells Konfigure how to configure environment to make
	// it possible for SOPS to find the keys
	sopsKeysDirEnvVar = "KONFIGURE_SOPS_KEYS_DIR"

	// sopsKeysSourceEnvVar tells Konfigure to either get keys from Kubernetes
	// Secrets or rely on local storage when setting up environment for SOPS
	sopsKeysSourceEnvVar = "KONFIGURE_SOPS_KEYS_SOURCE"
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
				fluxUpdaterConfig := fluxupdater.Config{
					CacheDir:                cacheDir,
					ApiServerHost:           os.Getenv(kubernetesServiceHostEnvVar),
					ApiServerPort:           os.Getenv(kubernetesServicePortEnvVar),
					SourceControllerService: os.Getenv(sourceServiceEnvVar),
					GitRepository:           os.Getenv(gitRepositoryEnvVar),
				}
				fluxUpdater, err := fluxupdater.New(fluxUpdaterConfig)
				if err != nil {
					return nil, microerror.Mask(err)
				}

				if err := fluxUpdater.UpdateConfig(); err != nil {
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

		var gen *service.Service
		{
			c := service.Config{
				VaultClient: vaultClient,

				Log:            r.logger,
				Dir:            dir,
				Installation:   installation,
				SOPSKeysDir:    os.Getenv(sopsKeysDirEnvVar),
				SOPSKeysSource: sopsKeysSource,
			}

			gen, err = service.New(c)
			if err != nil {
				return nil, microerror.Mask(err)
			}
		}

		in := service.GenerateInput{
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

func addNameSuffix(name string) string {
	if len(name) >= 63-len(nameSuffix)-1 {
		name = name[:63-len(nameSuffix)-1]
	}
	name = strings.TrimSuffix(name, "-")
	return fmt.Sprintf("%s-%s", name, nameSuffix)
}
