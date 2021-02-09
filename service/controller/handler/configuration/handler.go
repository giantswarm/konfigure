package configuration

import (
	"github.com/giantswarm/k8sclient/v5/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	vaultapi "github.com/hashicorp/vault/api"

	"github.com/giantswarm/config-controller/pkg/k8sresource"
)

const (
	Name = "configuration"
)

type Config struct {
	Logger micrologger.Logger

	K8sClient   k8sclient.Interface
	VaultClient *vaultapi.Client

	GitHubToken  string
	Installation string
	UniqueApp    bool
}

type Handler struct {
	logger micrologger.Logger

	k8sClient k8sclient.Interface
	resource  *k8sresource.Service

	installation string
	uniqueApp    bool
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

	var err error

	var resource *k8sresource.Service
	{
		c := k8sresource.Config{
			Client: config.K8sClient,
			Logger: config.Logger,
		}

		resource, err = k8sresource.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}

	}

	h := &Handler{
		logger: config.Logger,

		k8sClient: config.K8sClient,
		resource:  resource,

		installation: config.Installation,
		uniqueApp:    config.UniqueApp,
	}

	return h, nil
}

func (h *Handler) Name() string {
	return Name
}
