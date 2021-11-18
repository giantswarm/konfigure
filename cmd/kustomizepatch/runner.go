package kustomizepatch

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

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
	installationEnvVar  = "KONFIGURE_INSTALLATION"
	sourceServiceEnvVar = "SOURCE_SERVICE"
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

	var dir string // TODO(kuba): this is where giantswarm/config is pulled

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
		dir = "/home/jakub/Work/giantswarm/config"

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

		var gen *generator.Service
		{
			c := generator.Config{
				VaultClient: vaultClient,

				// TODO(kuba): r.config.Dir will be constant, probably. Remove it.
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

func addNameSuffix(name string) string {
	if len(name) >= 63-len(nameSuffix)-1 {
		name = name[:63-len(nameSuffix)-1]
	}
	name = strings.TrimSuffix(name, "-")
	return fmt.Sprintf("%s-%s", name, nameSuffix)
}
