package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	applicationv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/app/v7/pkg/app"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/imdario/mergo"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/konfigure/internal/generator"
	"github.com/giantswarm/konfigure/internal/meta"
	"github.com/giantswarm/konfigure/internal/vaultclient"
)

const (
	nameSuffix          = "konfigure"
	giantswarmNamespace = "giantswarm"
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
		var vaultClient *vaultclient.WrappedVaultClient
		{
			if r.flag.VaultSecretName != "" && r.flag.VaultSecretNamespace != "" {
				vaultClient, err = vaultclient.NewClientUsingK8sSecret(ctx, r.flag.VaultSecretNamespace, r.flag.VaultSecretName)
				if err != nil {
					return microerror.Mask(err)
				}
			}

			if vaultClient == nil {
				vaultClient, err = vaultclient.NewClientUsingEnv(ctx)
				if err != nil {
					return microerror.Mask(err)
				}
			}
		}

		var gen *generator.Service
		{
			c := generator.Config{
				VaultClient: vaultClient,

				Log:            r.logger,
				Dir:            r.flag.Dir,
				Installation:   r.flag.Installation,
				SOPSKeysDir:    r.flag.SOPSKeysDir,
				SOPSKeysSource: r.flag.SOPSKeysSource,
				Verbose:        r.flag.Verbose,
			}

			gen, err = generator.New(c)
			if err != nil {
				return microerror.Mask(err)
			}
		}

		in := generator.GenerateInput{
			App:       r.flag.AppName,
			Name:      addNameSuffix(r.flag.Name),
			Namespace: giantswarmNamespace,

			ExtraAnnotations: map[string]string{
				meta.Annotation.XAppInfo.Key():        meta.Annotation.XAppInfo.Val(r.flag.AppCatalog, r.flag.AppName, r.flag.AppVersion),
				meta.Annotation.XCreator.Key():        meta.Annotation.Default(),
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
			AppNamespace:        r.flag.AppDestinationNamespace,
			AppVersion:          r.flag.AppVersion,
			ConfigVersion:       configmap.Annotations[meta.Annotation.ConfigVersion.Key()],
			DisableForceUpgrade: r.flag.AppDisableForceUpgrade,
			Name:                r.flag.Name,
			InCluster:           true,
			Labels: map[string]string{
				meta.Label.ManagedBy.Key(): meta.Label.Default(),
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

	if r.flag.Raw {
		var err error

		var m1 map[string]interface{}
		err = yaml.Unmarshal([]byte(configmap.Data["configmap-values.yaml"]), &m1)
		if err != nil {
			return microerror.Mask(err)
		}

		var m2 map[string]interface{}
		err = yaml.Unmarshal([]byte(secret.Data["secret-values.yaml"]), &m2)
		if err != nil {
			return microerror.Mask(err)
		}

		err = mergo.Merge(&m1, m2)
		if err != nil {
			return microerror.Mask(err)
		}

		data, err := yaml.Marshal(m1)
		if err != nil {
			return microerror.Mask(err)
		}

		fmt.Printf("%s\n", data)

		return nil
	}

	if err := prettyPrint(configmap, false); err != nil {
		return microerror.Mask(err)
	}

	if err := prettyPrint(secret, false); err != nil {
		return microerror.Mask(err)
	}

	if err := prettyPrint(appCR, true); err != nil {
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

func prettyPrint(in interface{}, purgeStatus bool) error {
	if purgeStatus {
		bytes, err := json.Marshal(in)
		if err != nil {
			return microerror.Mask(err)
		}

		var m map[string]interface{}
		err = json.Unmarshal(bytes, &m)
		if err != nil {
			return microerror.Mask(err)
		}

		delete(m, "status")
		in = m
	}
	out, err := yaml.Marshal(in)
	if err != nil {
		return microerror.Mask(err)
	}

	fmt.Println("---")
	fmt.Printf("%s\n", out)
	return nil
}
