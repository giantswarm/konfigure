package configversion

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/apis/application/v1alpha1"
	"github.com/giantswarm/microerror"
	"k8s.io/apimachinery/pkg/types"

	"github.com/giantswarm/config-controller/pkg/k8sresource"
	"github.com/giantswarm/config-controller/service/controller/key"
)

func (h *Handler) EnsureCreated(ctx context.Context, obj interface{}) error {
	app, err := key.ToAppCR(obj)
	if err != nil {
		return microerror.Mask(err)
	}

	if app.Spec.Catalog == "" {
		h.logger.Debugf(ctx, "App CR has no .Spec.Catalog set")
		h.logger.Debugf(ctx, "cancelling handler")
		return nil
	}

	if app.Spec.Catalog == "releases" {
		if _, ok := k8sresource.GetAnnotation(&app, annotation.AppOperatorPaused); ok {
			h.logger.Debugf(ctx, "removing %#q annotation due to App from %#q catalog", annotation.AppOperatorPaused, app.Spec.Catalog)

			current := &v1alpha1.App{}
			modifyFunc := func() error {
				k8sresource.DeleteAnnotation(current, annotation.AppOperatorPaused)
				return nil
			}
			err := h.resource.Modify(ctx, k8sresource.ObjectKey(&app), current, modifyFunc, nil)
			if err != nil {
				return microerror.Mask(err)
			}

			h.logger.Debugf(ctx, "removed %#q annotation", annotation.AppOperatorPaused)
		}
		h.logger.Debugf(ctx, "cancelling handler due to App from %#q catalog", app.Spec.Catalog)
		return nil
	}

	h.logger.Debugf(ctx, "resolving config version from %#q catalog", app.Spec.Catalog)
	var index Index
	{
		indexYamlBytes, err := h.getCatalogIndex(ctx, app.Spec.Catalog)
		if err != nil {
			return microerror.Mask(err)
		}

		err = yaml.Unmarshal(indexYamlBytes, &index)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	entries, ok := index.Entries[app.Spec.Name]
	if !ok || len(entries) == 0 {
		h.logger.Debugf(ctx, "entries for App not found in %#q's index.yaml", app.Spec.Catalog)
		h.logger.Debugf(ctx, "cancelling handler")
		return nil
	}

	var configVersion string
	for _, entry := range entries {
		if entry.Version != app.Spec.Version {
			continue
		}

		if entry.Annotations == nil {
			configVersion = key.LegacyConfigVersion
		} else {
			v, ok := entry.Annotations[annotation.ConfigVersion]
			if ok {
				configVersion = v
			} else {
				configVersion = key.LegacyConfigVersion
			}
		}
		break
	}

	if configVersion == "" {
		h.logger.Debugf(ctx, "App has no entries matching version %#q in %#q's index.yaml", app.Spec.Version, app.Spec.Catalog)
		h.logger.Debugf(ctx, "cancelling handler")
		return nil
	}
	h.logger.Debugf(ctx, "resolved config version from %#q catalog to %#q", app.Spec.Catalog, configVersion)

	if v, ok := k8sresource.GetAnnotation(&app, annotation.ConfigVersion); ok {
		_, isPaused := k8sresource.GetAnnotation(&app, annotation.AppOperatorPaused)
		if v == configVersion && !isPaused {
			h.logger.Debugf(ctx, "cancelling handler due to App having config already set to %#q", v)
			return nil
		}
	}

	{
		h.logger.Debugf(ctx, "setting config version to %#q", configVersion)

		current := &v1alpha1.App{}
		modifyFunc := func() error {
			k8sresource.SetAnnotation(current, annotation.ConfigVersion, configVersion)
			return nil
		}
		err := h.resource.Modify(ctx, k8sresource.ObjectKey(&app), current, modifyFunc, nil)
		if err != nil {
			return microerror.Mask(err)
		}

		h.logger.Debugf(ctx, "set config version to %#q", configVersion)
	}

	return nil
}

func (h *Handler) getCatalogIndex(ctx context.Context, catalogName string) ([]byte, error) {
	client := &http.Client{}

	var catalog v1alpha1.AppCatalog
	{
		err := h.k8sClient.CtrlClient().Get(ctx, types.NamespacedName{Name: catalogName}, &catalog)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	url := strings.TrimRight(catalog.Spec.Storage.URL, "/") + "/index.yaml"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, &bytes.Buffer{}) // nolint: gosec
	if err != nil {
		return []byte{}, microerror.Mask(err)
	}
	response, err := client.Do(request)
	if err != nil {
		return []byte{}, microerror.Mask(err)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return []byte{}, microerror.Mask(err)
	}

	return body, nil
}
