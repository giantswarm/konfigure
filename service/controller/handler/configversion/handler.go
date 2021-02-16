package configversion

import (
	"github.com/giantswarm/k8sclient/v5/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"

	"github.com/giantswarm/config-controller/internal/generator/github"
	"github.com/giantswarm/config-controller/pkg/k8sresource"
)

const (
	Name = "configversion"
)

type Config struct {
	K8sClient k8sclient.Interface
	Logger    micrologger.Logger

	GitHubToken string
}

type Handler struct {
	k8sClient k8sclient.Interface
	logger    micrologger.Logger

	gitHub   *github.GitHub
	resource *k8sresource.Service
}

func New(config Config) (*Handler, error) {
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}
	if config.K8sClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.K8sClient must not be empty", config)
	}
	if config.GitHubToken == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.GitHubToken must not be empty", config)
	}

	var err error

	var gh *github.GitHub
	{
		c := github.Config{
			Token: config.GitHubToken,
		}

		gh, err = github.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

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
		k8sClient: config.K8sClient,
		logger:    config.Logger,

		gitHub:   gh,
		resource: resource,
	}

	return h, nil
}

func (h *Handler) Name() string {
	return Name
}
