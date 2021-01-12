package values

import (
	"context"

	"github.com/giantswarm/k8sclient/v5/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	vaultapi "github.com/hashicorp/vault/api"
	corev1 "k8s.io/api/core/v1"

	"github.com/giantswarm/config-controller/pkg/decrypt"
	"github.com/giantswarm/config-controller/pkg/generator"
	"github.com/giantswarm/config-controller/pkg/generator/key"
	controllerkey "github.com/giantswarm/config-controller/service/controller/key"
	"github.com/giantswarm/config-controller/service/internal/github"
)

const (
	Name       = "values"
	ConfigRepo = "config"
)

type Config struct {
	K8sClient k8sclient.Interface
	Logger    micrologger.Logger

	GitHubToken    string
	Installation   string
	ProjectVersion string
	VaultClient    *vaultapi.Client
}

type Handler struct {
	k8sClient k8sclient.Interface
	logger    micrologger.Logger

	decryptTraverser *decrypt.YAMLTraverser
	gitHub           *github.GitHub
	installation     string
	projectVersion   string
}

func New(config Config) (*Handler, error) {
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}
	if config.K8sClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.K8sClient must not be empty", config)
	}
	if config.VaultClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.VaultClient must not be empty", config)
	}
	if config.GitHubToken == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.GitHubToken must not be empty", config)
	}
	if config.Installation == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Installation must not be empty", config)
	}

	var decryptTraverser *decrypt.YAMLTraverser
	{
		c := decrypt.VaultDecrypterConfig{
			VaultClient: config.VaultClient,
		}

		decrypter, err := decrypt.NewVaultDecrypter(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		decryptTraverser, err = decrypt.NewYAMLTraverser(
			decrypt.YAMLTraverserConfig{
				Decrypter: decrypter,
			},
		)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	gh, err := github.New(github.Config{
		GitHubToken: config.GitHubToken,
	})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	h := &Handler{
		k8sClient:        config.K8sClient,
		logger:           config.Logger,
		decryptTraverser: decryptTraverser,
		gitHub:           gh,
		installation:     config.Installation,
		projectVersion:   config.ProjectVersion,
	}

	return h, nil
}

func (h *Handler) Name() string {
	return Name
}

func (h *Handler) generateConfig(ctx context.Context, installation, namespace, app, configVersion string) (configmap *corev1.ConfigMap, secret *corev1.Secret, err error) {
	var store generator.Filesystem
	var ref string
	{
		if configVersion == "" {
			return nil, nil, microerror.Maskf(executionFailedError, "configVersion must be defined")
		}

		tagReference := controllerkey.TryVersionToTag(configVersion)
		if tagReference != "" {
			tag, err := h.gitHub.GetLatestTag(ctx, key.Owner, ConfigRepo, tagReference)
			if err != nil {
				return nil, nil, microerror.Mask(err)
			}

			store, err = h.gitHub.GetFilesByTag(ctx, key.Owner, ConfigRepo, tag)
			if err != nil {
				return nil, nil, microerror.Mask(err)
			}
			ref = tag
		} else {
			store, err = h.gitHub.GetFilesByBranch(ctx, key.Owner, ConfigRepo, configVersion)
			if err != nil {
				return nil, nil, microerror.Mask(err)
			}
			ref = configVersion
		}
	}

	gen, err := generator.New(&generator.Config{
		Fs:               store,
		DecryptTraverser: h.decryptTraverser,
		ProjectVersion:   h.projectVersion,
	})
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	configmap, secret, err = gen.GenerateConfig(ctx, installation, app, namespace, ref)
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	return configmap, secret, nil
}
