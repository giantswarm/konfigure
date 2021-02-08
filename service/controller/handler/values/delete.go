package values

import (
	"context"

	"github.com/giantswarm/apiextensions/v3/pkg/annotation"
	"github.com/giantswarm/apiextensions/v3/pkg/apis/application/v1alpha1"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/config-controller/service/controller/key"
)

func (h *Handler) EnsureDeleted(ctx context.Context, obj interface{}) error {
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
		h.logger.Debugf(ctx, "cancelling handler")
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Spec.Config.ConfigMap.Name,
			Namespace: app.Spec.Config.ConfigMap.Namespace,
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Spec.Config.Secret.Name,
			Namespace: app.Spec.Config.Secret.Namespace,
		},
	}
	if cm.Name == "" || secret.Name == "" {
		ref := configVersion
		if tagRef := key.TryVersionToTag(configVersion); tagRef != "" {
			ref = tagRef
		}
		name := generateResourceName(app.Spec.Name, ref)
		if cm.Name == "" {
			cm.Name = name
			cm.Namespace = app.Namespace
		}
		if secret.Name == "" {
			secret.Name = name
			secret.Namespace = app.Namespace
		}
	}

	h.logger.Debugf(ctx, "deleting App config version %#q", configVersion)
	h.logger.Debugf(ctx, "clearing App config version %#q configmap and secret details", configVersion)
	app.Spec.Config = v1alpha1.AppSpecConfig{}
	err = h.k8sClient.CtrlClient().Update(ctx, &app)
	if err != nil {
		return microerror.Mask(err)
	}
	h.logger.Debugf(ctx, "cleared App config version %#q configmap and secret details", configVersion)

	h.logger.Debugf(ctx, "deleting configmap for App, config version %#q", configVersion)
	err = h.k8sClient.CtrlClient().Delete(ctx, cm)
	if client.IgnoreNotFound(err) != nil {
		return microerror.Mask(err)
	}
	h.logger.Debugf(ctx, "deleted configmap for App, config version %#q", configVersion)

	h.logger.Debugf(ctx, "deleting secret for App, config version %#q", configVersion)
	err = h.k8sClient.CtrlClient().Delete(ctx, secret)
	if client.IgnoreNotFound(err) != nil {
		return microerror.Mask(err)
	}
	h.logger.Debugf(ctx, "deleted secret for App, config version %#q", configVersion)

	h.logger.Debugf(ctx, "clearing %q annotation from App CR", annotation.AppOperatorPaused)
	app.SetAnnotations(key.RemoveAnnotation(app.GetAnnotations(), annotation.AppOperatorPaused))
	err = h.k8sClient.CtrlClient().Update(ctx, &app)
	if err != nil {
		return microerror.Mask(err)
	}
	h.logger.Debugf(ctx, "cleared %q annotation from App CR", annotation.AppOperatorPaused)

	h.logger.Debugf(ctx, "deleted App config version %#q", configVersion)

	return nil
}
