package configuration

import (
	"reflect"

	"github.com/giantswarm/apiextensions/v3/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/k8sclient/v5/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	vaultapi "github.com/hashicorp/vault/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/config-controller/internal/configversion"
	"github.com/giantswarm/config-controller/internal/generator"
	"github.com/giantswarm/config-controller/internal/meta"

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

	configVersion *configversion.Service
	generator     *generator.Service
	k8sClient     k8sclient.Interface
	resource      *k8sresource.Service

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

	var configVersion *configversion.Service
	{
		c := configversion.Config{
			K8sClient: config.K8sClient,
		}

		configVersion, err = configversion.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var gen *generator.Service
	{
		c := generator.Config{
			VaultClient: config.VaultClient,

			GitHubToken:  config.GitHubToken,
			Installation: config.Installation,
		}

		gen, err = generator.New(c)
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
		logger: config.Logger,

		configVersion: configVersion,
		generator:     gen,
		k8sClient:     config.K8sClient,
		resource:      resource,

		installation: config.Installation,
		uniqueApp:    config.UniqueApp,
	}

	return h, nil
}

func (h *Handler) Name() string {
	return Name
}

func getConfigObjectsMeta(config *v1alpha1.Config) (current, orphaned []k8sresource.Object, err error) {
	currConfig := config.Status.Config
	prevConfig, err := meta.Annotation.XPreviousConfig.Get(config)
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	if obj := getConfigMapMeta(currConfig); obj.Name != "" {
		current = append(current, obj)
	}
	if obj := getSecretMeta(currConfig); obj.Name != "" {
		current = append(current, obj)
	}
	if obj := getConfigMapMeta(prevConfig); obj.Name != "" && !reflect.DeepEqual(currConfig.ConfigMapRef, prevConfig.ConfigMapRef) {
		orphaned = append(orphaned, obj)
	}
	if obj := getSecretMeta(prevConfig); obj.Name != "" && !reflect.DeepEqual(currConfig.SecretRef, prevConfig.SecretRef) {
		orphaned = append(orphaned, obj)
	}

	return current, orphaned, err
}

func getConfigMapMeta(c v1alpha1.ConfigStatusConfig) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.ConfigMapRef.Name,
			Namespace: c.ConfigMapRef.Namespace,
		},
	}
}

func getSecretMeta(c v1alpha1.ConfigStatusConfig) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.SecretRef.Name,
			Namespace: c.SecretRef.Namespace,
		},
	}
}
