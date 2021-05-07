package generate

import (
	"context"
	"fmt"
	"io"

	"github.com/ghodss/yaml"
	applicationv1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/application/v1alpha1"
	"github.com/giantswarm/app/v4/pkg/app"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"

	"github.com/giantswarm/konfigure/internal/generator"
	"github.com/giantswarm/konfigure/internal/meta"
)

type runner struct {
	flag   *flag
	logger micrologger.Logger
	stdout io.Writer
	stderr io.Writer
}

func (r *runner) Run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	err := r.flag.Validate()
	if err != nil {
		return microerror.Mask(err)
	}

	err = r.run(ctx, cmd, args)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (r *runner) run(ctx context.Context, cmd *cobra.Command, args []string) error {
	var err error

	var configmap *corev1.ConfigMap
	var secret *corev1.Secret
	{
		var vaultClient *vaultapi.Client
		{
			vaultClient, err = createVaultClientUsingEnv(ctx)
			if err != nil {
				return microerror.Mask(err)
			}
		}

		var gen *generator.Service
		{
			c := generator.Config{
				VaultClient: vaultClient,

				Dir:          r.flag.Dir,
				Installation: r.flag.Installation,
				Verbose:      r.flag.Verbose,
			}

			gen, err = generator.New(c)
			if err != nil {
				return microerror.Mask(err)
			}
		}

		in := generator.GenerateInput{
			App:       r.flag.AppName,
			Name:      r.flag.Name,
			Namespace: r.flag.Namespace,

			ExtraAnnotations: map[string]string{
				meta.Annotation.XAppInfo.Key():        meta.Annotation.XAppInfo.Val(r.flag.AppCatalog, r.flag.AppName, r.flag.AppVersion),
				meta.Annotation.XCreator.Key():        meta.Annotation.XCreator.Default(),
				meta.Annotation.XInstallation.Key():   r.flag.Installation,
				meta.Annotation.XProjectVersion.Key(): meta.Annotation.XProjectVersion.Val(false),
			},
			ExtraLabels: nil,
		}

		configmap, secret, err = gen.Generate(ctx, in)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	var appCR *applicationv1alpha1.App
	{
		c := app.Config{
			AppCatalog:          r.flag.AppCatalog,
			AppName:             r.flag.AppName,
			AppNamespace:        r.flag.Namespace,
			AppVersion:          r.flag.AppVersion,
			ConfigVersion:       configmap.Annotations[meta.Annotation.ConfigVersion.Key()],
			DisableForceUpgrade: r.flag.AppDisableForceUpgrade,
			Name:                r.flag.Name,
			UserConfigMapName:   configmap.Name,
			UserSecretName:      secret.Name,
		}

		appCR = app.NewCR(c)
	}

	if r.flag.Raw {
		fmt.Println("---")
		fmt.Printf(string(configmap.Data["configmap-values.yaml"]) + "\n")
		fmt.Println("---")
		fmt.Printf(string(secret.Data["secret-values.yaml"]) + "\n")
		return nil
	}

	if err := prettyPrint(configmap); err != nil {
		return microerror.Mask(err)
	}

	if err := prettyPrint(secret); err != nil {
		return microerror.Mask(err)
	}

	if err := prettyPrint(appCR); err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func prettyPrint(in interface{}) error {
	fmt.Println("---")

	out, err := yaml.Marshal(in)
	if err != nil {
		return microerror.Mask(err)
	}

	fmt.Printf(string(out) + "\n")
	return nil
}
