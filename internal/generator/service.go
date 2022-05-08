package generator

import (
	"context"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	vaultapi "github.com/hashicorp/vault/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/konfigure/internal/meta"
	"github.com/giantswarm/konfigure/internal/sopsenv"
	"github.com/giantswarm/konfigure/pkg/decrypt"
	"github.com/giantswarm/konfigure/pkg/filesystem"
	"github.com/giantswarm/konfigure/pkg/generator"
	"github.com/giantswarm/konfigure/pkg/xstrings"
)

type Config struct {
	Log         micrologger.Logger
	VaultClient *vaultapi.Client

	Dir            string
	Installation   string
	SOPSKeysDir    string
	SOPSKeysSource string
	Verbose        bool
}

type Service struct {
	log              micrologger.Logger
	decryptTraverser generator.DecryptTraverser

	sopsKeysDir    string
	sopsKeysSource string

	dir          string
	installation string
	verbose      bool
}

func New(config Config) (*Service, error) {
	if config.VaultClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.VaultClient must not be empty", config)
	}

	if config.Installation == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Installation must not be empty", config)
	}

	if config.Dir == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Dir must not be empty", config)
	}

	var err error

	var decrypter *decrypt.VaultDecrypter
	{
		c := decrypt.VaultDecrypterConfig{
			VaultClient: config.VaultClient,
		}

		decrypter, err = decrypt.NewVaultDecrypter(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var decryptTraverser *decrypt.YAMLTraverser
	{
		c := decrypt.YAMLTraverserConfig{
			Decrypter: decrypter,
		}

		decryptTraverser, err = decrypt.NewYAMLTraverser(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

	}

	s := &Service{
		log:              config.Log,
		decryptTraverser: decryptTraverser,

		sopsKeysDir:    config.SOPSKeysDir,
		sopsKeysSource: config.SOPSKeysSource,

		dir:          config.Dir,
		installation: config.Installation,
		verbose:      config.Verbose,
	}

	return s, nil
}

type GenerateInput struct {
	// App for which the configuration is generated.
	App string

	// Name of the generated ConfigMap and Secret.
	Name string
	// Namespace of the generated ConfigMap and Secret.
	Namespace string

	// ExtraAnnotations are additional annotations to be set on the
	// generated ConfigMap and Secret. By default
	// "config.giantswarm.io/version" annotation is set.
	ExtraAnnotations map[string]string
	// ExtraLabels are additional labels to be set on the generated
	// ConfigMap and Secret.
	ExtraLabels map[string]string
	// VersionOverride allows user to set version manually.
	VersionOverride string
}

func (s *Service) Generate(ctx context.Context, in GenerateInput) (configmap *corev1.ConfigMap, secret *corev1.Secret, err error) {
	store := &filesystem.Store{
		Dir: s.dir,
	}

	var gen *generator.Generator
	{
		c := generator.Config{
			Fs:               store,
			DecryptTraverser: s.decryptTraverser,

			Installation: s.installation,
			Verbose:      s.verbose,
		}

		gen, err = generator.New(c)
		if err != nil {
			return nil, nil, microerror.Mask(err)
		}
	}

	var version string
	if in.VersionOverride != "" {
		version = in.VersionOverride
	} else {
		v, err := store.Version()
		if err != nil {
			return nil, nil, microerror.Mask(err)
		}
		version = v
	}

	annotations := xstrings.CopyMap(in.ExtraAnnotations)
	annotations[meta.Annotation.ConfigVersion.Key()] = version

	meta := metav1.ObjectMeta{
		Name:      in.Name,
		Namespace: in.Namespace,

		Annotations: annotations,
		Labels:      in.ExtraLabels,
	}

	var sopsCleanup func()
	var sopsEnv *sopsenv.SOPSEnv
	{
		c := sopsenv.SOPSEnvConfig{
			KeysDir:    s.sopsKeysDir,
			KeysSource: s.sopsKeysSource,
		}

		sopsEnv, sopsCleanup, err = sopsenv.NewSOPSEnv(c)
		if err != nil {
			return nil, nil, microerror.Mask(err)
		}

		if sopsCleanup != nil {
			defer sopsCleanup()
		}
	}

	err = sopsEnv.Setup(ctx)
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	configMap, secret, err := gen.GenerateConfig(ctx, in.App, meta)
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	return configMap, secret, nil
}
