package values

import (
	"context"
	"reflect"

	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/apis/application/v1alpha1"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/config-controller/service/controller/key"
)

func (h *Handler) EnsureCreated(ctx context.Context, obj interface{}) error {
	app, err := key.ToAppCR(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	annotations := app.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	configVersion, ok := annotations[annotation.ConfigVersion]
	if !ok {
		h.logger.Debugf(ctx, "App CR is missing %q annotation", annotation.ConfigVersion)
		h.logger.Debugf(ctx, "cancelling handler")
		return nil
	}

	if configVersion == key.LegacyConfigVersion {
		h.logger.Debugf(ctx, "App CR has config version %#q", configVersion)
		if _, ok := annotations[annotation.AppOperatorPaused]; ok {
			h.logger.Debugf(ctx, "App does not use generated config, removing pause annotation")
			app.SetAnnotations(key.RemoveAnnotation(annotations, annotation.AppOperatorPaused))
			err = h.k8sClient.CtrlClient().Update(ctx, &app)
			if err != nil {
				return microerror.Mask(err)
			}
			h.logger.Debugf(ctx, "removed %#q annotation", annotation.AppOperatorPaused)
		}
		h.logger.Debugf(ctx, "cancelling handler")
		return nil
	}

	h.logger.Debugf(ctx, "generating app config version %#q", configVersion)
	configmap, secret, err := h.generateConfig(ctx, h.installation, app.Namespace, app.Spec.Name, configVersion)
	if err != nil {
		return microerror.Mask(err)
	}
	h.logger.Debugf(ctx, "generated app config version %#q", configVersion)

	h.logger.Debugf(ctx, "ensuring configmap %s/%s", configmap.Namespace, configmap.Name)
	err = h.k8sClient.CtrlClient().Create(ctx, configmap)
	if apierrors.IsAlreadyExists(err) {
		err = h.k8sClient.CtrlClient().Update(ctx, configmap)
		if err != nil {
			return microerror.Mask(err)
		}
	} else if err != nil {
		return microerror.Mask(err)
	}
	h.logger.Debugf(ctx, "ensured configmap %s/%s", configmap.Namespace, configmap.Name)

	h.logger.Debugf(ctx, "ensuring secret %s/%s", secret.Namespace, secret.Name)
	err = h.k8sClient.CtrlClient().Create(ctx, secret)
	if apierrors.IsAlreadyExists(err) {
		err = h.k8sClient.CtrlClient().Update(ctx, secret)
		if err != nil {
			return microerror.Mask(err)
		}
	} else if err != nil {
		return microerror.Mask(err)
	}
	h.logger.Debugf(ctx, "ensured secret %s/%s", secret.Namespace, secret.Name)

	configmapReference := v1alpha1.AppSpecConfigConfigMap{
		Namespace: configmap.Namespace,
		Name:      configmap.Name,
	}
	secretReference := v1alpha1.AppSpecConfigSecret{
		Namespace: secret.Namespace,
		Name:      secret.Name,
	}
	if reflect.DeepEqual(app.Spec.Config.ConfigMap, configmapReference) && reflect.DeepEqual(app.Spec.Config.Secret, secretReference) {
		h.logger.Debugf(ctx, "configmap and secret are up to date")
		return nil
	}

	if app.Spec.Config.ConfigMap.Name != "" {
		h.logger.Debugf(ctx, "deleting configmap %#q in %#q namespace for older version", configmap.Name, configmap.Namespace)
		err = h.k8sClient.CtrlClient().Delete(
			ctx,
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: configmap.Namespace,
					Name:      configmap.Name,
				},
			},
		)
		if client.IgnoreNotFound(err) != nil {
			return microerror.Mask(err)
		}
		h.logger.Debugf(ctx, "deleted configmap %#q in %#q namespace for older version", configmap.Name, configmap.Namespace)
	}

	if app.Spec.Config.Secret.Name != "" {
		h.logger.Debugf(ctx, "deleting secret %#q in %#q namespace for older version", secret.Name, secret.Namespace)
		err = h.k8sClient.CtrlClient().Delete(
			ctx,
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: secret.Namespace,
					Name:      secret.Name,
				},
			},
		)
		if client.IgnoreNotFound(err) != nil {
			return microerror.Mask(err)
		}
		h.logger.Debugf(ctx, "deleted secret %#q in %#q namespace for older version", secret.Name, secret.Namespace)
	}

	h.logger.Debugf(ctx, "updating App CR with configmap and secret details")
	app.SetAnnotations(key.RemoveAnnotation(annotations, annotation.AppOperatorPaused))
	app.Spec.Config.ConfigMap = configmapReference
	app.Spec.Config.Secret = secretReference
	err = h.k8sClient.CtrlClient().Update(ctx, &app)
	if err != nil {
		return microerror.Mask(err)
	}
	h.logger.Debugf(ctx, "updated App CR with configmap and secret details")

	return nil
}
