package configuration

import (
	"context"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/config-controller/pkg/k8sresource"
	"github.com/giantswarm/config-controller/service/controller/key"
)

func (h *Handler) EnsureDeleted(ctx context.Context, obj interface{}) error {
	config, err := key.ToConfigCR(obj)
	if err != nil {
		return microerror.Mask(err)
	}

	var toDelete []k8sresource.Object
	{
		current, orphaned, err := getConfigObjectsMeta(config)
		if err != nil {
			return microerror.Mask(err)
		}

		for _, obj := range current {
			h.logger.Debugf(ctx, "found %#q %#q", h.resource.Kind(obj), k8sresource.ObjectKey(obj))
		}
		for _, obj := range orphaned {
			h.logger.Debugf(ctx, "found orphaned %#q %#q", h.resource.Kind(obj), k8sresource.ObjectKey(obj))
		}

		toDelete = append(current, orphaned...)
	}

	if len(toDelete) == 0 {
		h.logger.Debugf(ctx, "cancelling handler due to no found objects to cleanup")
	}

	// Cleanup.
	for _, o := range toDelete {
		err := h.resource.EnsureDeleted(ctx, o)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}
