// Package service implements business logic to create Kubernetes resources
// against the Kubernetes API.
package service

import (
	"context"
	"sync"

	"github.com/giantswarm/apiextensions/v3/pkg/apis/application/v1alpha1"
	"github.com/giantswarm/k8sclient/v5/pkg/k8sclient"
	"github.com/giantswarm/k8sclient/v5/pkg/k8srestconfig"
	"github.com/giantswarm/microendpoint/service/version"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/spf13/viper"
	"k8s.io/client-go/rest"

	"github.com/giantswarm/config-controller/flag"
	"github.com/giantswarm/config-controller/pkg/project"
	"github.com/giantswarm/config-controller/service/collector"
	"github.com/giantswarm/config-controller/service/controller"
)

// Config represents the configuration used to create a new service.
type Config struct {
	Logger micrologger.Logger

	Flag  *flag.Flag
	Viper *viper.Viper
}

type Service struct {
	Version *version.Service

	bootOnce                  sync.Once
	appCatalogEntryController *controller.AppCatalogEntry
	appController             *controller.App
	operatorCollector         *collector.Set
}

// New creates a new configured service object.
func New(config Config) (*Service, error) {
	var serviceAddress string
	// Settings.
	if config.Flag == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Flag must not be empty")
	}
	if config.Viper == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Viper must not be empty")
	}
	if config.Flag.Service.Kubernetes.KubeConfig == "" {
		serviceAddress = config.Viper.GetString(config.Flag.Service.Kubernetes.Address)
	} else {
		serviceAddress = ""
	}

	// Dependencies.
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "logger must not be empty")
	}

	var err error

	var restConfig *rest.Config
	{
		c := k8srestconfig.Config{
			Logger: config.Logger,

			Address:    serviceAddress,
			InCluster:  config.Viper.GetBool(config.Flag.Service.Kubernetes.InCluster),
			KubeConfig: config.Viper.GetString(config.Flag.Service.Kubernetes.KubeConfig),
			TLS: k8srestconfig.ConfigTLS{
				CAFile:  config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CAFile),
				CrtFile: config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CrtFile),
				KeyFile: config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.KeyFile),
			},
		}

		restConfig, err = k8srestconfig.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var k8sClient k8sclient.Interface
	{
		c := k8sclient.ClientsConfig{
			Logger:     config.Logger,
			RestConfig: restConfig,
			SchemeBuilder: k8sclient.SchemeBuilder{
				v1alpha1.AddToScheme,
			},
		}

		k8sClient, err = k8sclient.NewClients(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var vaultClient *vaultapi.Client
	{
		c := vaultapi.DefaultConfig()
		c.Address = config.Viper.GetString(config.Flag.Service.Vault.Address)
		vaultClient, err = vaultapi.NewClient(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		vaultClient.SetToken(config.Viper.GetString(config.Flag.Service.Vault.Token))
	}

	var appCatalogEntryController *controller.AppCatalogEntry
	{
		c := controller.AppCatalogEntryConfig{
			K8sClient: k8sClient,
			Logger:    config.Logger,

			GitHubToken: config.Viper.GetString(config.Flag.Service.GitHub.Token),
			UniqueApp:   config.Viper.GetBool(config.Flag.Service.App.Unique),
		}

		appCatalogEntryController, err = controller.NewAppCatalogEntry(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var appController *controller.App
	{
		c := controller.AppConfig{
			K8sClient: k8sClient,
			Logger:    config.Logger,

			GitHubToken:  config.Viper.GetString(config.Flag.Service.GitHub.Token),
			Installation: config.Viper.GetString(config.Flag.Service.Installation.Name),
			UniqueApp:    config.Viper.GetBool(config.Flag.Service.App.Unique),
			VaultClient:  vaultClient,
		}

		appController, err = controller.NewApp(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var operatorCollector *collector.Set
	{
		c := collector.SetConfig{
			K8sClient: k8sClient.K8sClient(),
			Logger:    config.Logger,
		}

		operatorCollector, err = collector.NewSet(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var versionService *version.Service
	{
		c := version.Config{
			Description: project.Description(),
			GitCommit:   project.GitSHA(),
			Name:        project.Name(),
			Source:      project.Source(),
			Version:     project.Version(),
		}

		versionService, err = version.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	s := &Service{
		Version: versionService,

		bootOnce:                  sync.Once{},
		appController:             appController,
		appCatalogEntryController: appCatalogEntryController,
		operatorCollector:         operatorCollector,
	}

	return s, nil
}

func (s *Service) Boot(ctx context.Context) {
	s.bootOnce.Do(func() {
		go s.operatorCollector.Boot(ctx) // nolint:errcheck

		go s.appCatalogEntryController.Boot(ctx)
		go s.appController.Boot(ctx)
	})
}
