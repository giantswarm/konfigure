package configversion

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"

	applicationv1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/application/v1alpha1"
	corev1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/k8sclient/v5/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/config-controller/internal/meta"
)

type Config struct {
	K8sClient k8sclient.Interface
}

type Service struct {
	k8sClient k8sclient.Interface
}

func New(config Config) (*Service, error) {
	if config.K8sClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.K8sClient must not be empty", config)
	}

	s := &Service{
		k8sClient: config.K8sClient,
	}

	return s, nil
}

// Get tries to resolve config version for the app version.
//
// It returns error matched by IsNotFound if the app version does not have
// "config.giantswarm.io/version" annotation in the catalog.
func (s *Service) Get(ctx context.Context, app corev1alpha1.ConfigSpecApp) (string, error) {
	if app.Catalog == "releases" {
		return "", microerror.Maskf(executionFailedError, "catalog %#q is not supported", app.Catalog)
	}

	index, err := s.getCatalogIndex(ctx, app.Catalog)
	if err != nil {
		return "", microerror.Mask(err)
	}

	entries, ok := index.Entries[app.Name]
	if !ok || len(entries) == 0 {
		return "", microerror.Maskf(executionFailedError, "App %#q not found in catalog %#q", app.Name, app.Catalog)
	}

	appVersionFound := false
	for _, entry := range entries {
		if entry.Version != app.Version {
			continue
		}

		appVersionFound = true

		if entry.Annotations == nil {
			break
		}

		v, ok := entry.Annotations[meta.Annotation.ConfigVersion.Key()]
		if !ok {
			break
		}

		return v, nil
	}

	av := app.Name + "@" + app.Version

	if appVersionFound {
		return "", microerror.Maskf(
			notFoundError,
			"annotation %#q not found for App %#q not found in catalog %#q",
			meta.Annotation.ConfigVersion.Key(), av, app.Catalog,
		)
	}

	return "", microerror.Maskf(executionFailedError, "App %#q not found in catalog %#q", av, app.Catalog)
}

func (s *Service) getCatalogIndex(ctx context.Context, catalogName string) (catalogIndex, error) {
	client := &http.Client{}

	var catalog applicationv1alpha1.AppCatalog
	{
		err := s.k8sClient.CtrlClient().Get(ctx, types.NamespacedName{Name: catalogName}, &catalog)
		if err != nil {
			return catalogIndex{}, microerror.Mask(err)
		}
	}

	url := strings.TrimRight(catalog.Spec.Storage.URL, "/") + "/index.yaml"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, &bytes.Buffer{}) // nolint: gosec
	if err != nil {
		return catalogIndex{}, microerror.Mask(err)
	}
	response, err := client.Do(request)
	if err != nil {
		return catalogIndex{}, microerror.Mask(err)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return catalogIndex{}, microerror.Mask(err)
	}

	var index catalogIndex
	err = yaml.Unmarshal(body, &index)
	if err != nil {
		return catalogIndex{}, microerror.Mask(err)
	}

	return index, nil
}
