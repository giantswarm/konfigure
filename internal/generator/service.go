package generator

import (
	"context"

	"github.com/giantswarm/microerror"
	vaultapi "github.com/hashicorp/vault/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/config-controller/internal/meta"
	"github.com/giantswarm/config-controller/pkg/decrypt"
	"github.com/giantswarm/config-controller/pkg/generator"
	"github.com/giantswarm/config-controller/pkg/github"
)

type Config struct {
	VaultClient *vaultapi.Client

	GitHubToken  string
	Installation string
}

type Service struct {
	decryptTraverser generator.DecryptTraverser
	gitHub           *github.GitHub

	installation string
}

func New(config Config) (*Service, error) {
	if config.VaultClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.VaultClient must not be empty", config)
	}

	if config.GitHubToken == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.GitHubToken must not be empty", config)
	}
	if config.Installation == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Installation must not be empty", config)
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

	var gitHub *github.GitHub
	{
		c := github.Config{
			Token: config.GitHubToken,
		}

		gitHub, err = github.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	s := &Service{
		decryptTraverser: decryptTraverser,
		gitHub:           gitHub,

		installation: config.Installation,
	}

	return s, nil
}

type GenerateInput struct {
	// App for which the configuration is generated.
	App string
	// ConfigVersion used to generate the configuration which is either a major
	// version range in format "2.x.x" or a branch name. Exact version
	// names (e.g. "1.2.3" are not supported.
	ConfigVersion string

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
}

func (s *Service) Generate(ctx context.Context, in GenerateInput) (configmap *corev1.ConfigMap, secret *corev1.Secret, err error) {
	tagPrefix, isTagRange, err := toTagPrefix(in.ConfigVersion)
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	const (
		owner = "giantswarm"
		repo  = "config"
	)

	var store github.Store
	if isTagRange {
		tag, err := s.gitHub.GetLatestTag(ctx, owner, repo, tagPrefix)
		if err != nil {
			return nil, nil, microerror.Mask(err)
		}

		store, err = s.gitHub.GetFilesByTag(ctx, owner, repo, tag)
		if err != nil {
			return nil, nil, microerror.Mask(err)
		}
	} else {
		branch := in.ConfigVersion

		store, err = s.gitHub.GetFilesByBranch(ctx, owner, repo, branch)
		if err != nil {
			return nil, nil, microerror.Mask(err)
		}
	}

	var gen *generator.Generator
	{
		c := generator.Config{
			Fs:               store,
			DecryptTraverser: s.decryptTraverser,

			Installation: s.installation,
		}

		gen, err = generator.New(c)
		if err != nil {
			return nil, nil, microerror.Mask(err)
		}
	}

	annotations := copyMap(in.ExtraAnnotations)
	annotations[meta.Annotation.ConfigVersion.Key()] = in.ConfigVersion

	meta := metav1.ObjectMeta{
		Name:      in.Name,
		Namespace: in.Namespace,

		Annotations: annotations,
		Labels:      in.ExtraLabels,
	}

	configMap, secret, err := gen.GenerateConfig(ctx, in.App, meta)
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	return configMap, secret, nil
}

func copyMap(m map[string]string) map[string]string {
	n := make(map[string]string, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}
