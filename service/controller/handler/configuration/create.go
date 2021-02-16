package configuration

import (
	"context"
	"crypto/sha1" // nolint:gosec
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/giantswarm/apiextensions/v3/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/config-controller/internal/configversion"
	"github.com/giantswarm/config-controller/internal/generator"
	"github.com/giantswarm/config-controller/internal/meta"
	"github.com/giantswarm/config-controller/pkg/k8sresource"
	"github.com/giantswarm/config-controller/pkg/xstrings"
	"github.com/giantswarm/config-controller/service/controller/key"
)

func (h *Handler) EnsureCreated(ctx context.Context, obj interface{}) error {
	config, err := key.ToConfigCR(obj)
	if err != nil {
		return microerror.Mask(err)
	}

	var configVersion string
	{
		cav := config.Spec.App.Catalog + "/" + config.Spec.App.Name + "@" + config.Spec.App.Version

		h.logger.Debugf(ctx, "resolving config version for App %#q", cav)

		configVersion, err = h.configVersion.Get(ctx, config.Spec.App)
		if configversion.IsNotFound(err) {
			h.logger.Debugf(ctx, "configuration version not found for App %#q, falling back to draughtsman", cav)
			configVersion = ""
		} else if err != nil {
			return microerror.Mask(err)
		} else {
			h.logger.Debugf(ctx, "resolved config version %#q for App %#q", configVersion, cav)
		}
	}

	var configmap *corev1.ConfigMap
	var secret *corev1.Secret
	{
		name, err := genStableObjectName(config)
		if err != nil {
			return microerror.Mask(err)
		}

		namespace := config.Namespace

		generateIn := generator.GenerateInput{
			App:           config.Spec.App.Name,
			ConfigVersion: configVersion,

			Name:      name,
			Namespace: namespace,

			ExtraAnnotations: map[string]string{
				meta.Annotation.XAppInfo.Key():        meta.Annotation.XAppInfo.ValFromConfig(config),
				meta.Annotation.XInstallation.Key():   h.installation,
				meta.Annotation.XProjectVersion.Key(): meta.Annotation.XProjectVersion.Val(h.uniqueApp),
			},
			ExtraLabels: map[string]string{
				meta.Label.ManagedBy.Key(): meta.Label.ManagedBy.Default(),
			},
		}

		nn := namespace + "/" + name

		if configVersion == "" {
			h.logger.Debugf(ctx, "copying %#q ConfigMap and Secret from draughtsman", nn)

			generateIn.ConfigVersion = "draughtsman" // So it appears in the annotation.

			configmap, secret, err = h.newConfigurationFromDraughtsman(ctx, generateIn)
			if err != nil {
				return microerror.Mask(err)
			}

			h.logger.Debugf(ctx, "copied %#q ConfigMap and Secret from draughtsman", nn)
		} else {
			h.logger.Debugf(ctx, "generating %#q ConfigMap and Secret for config version %#q", nn, configVersion)

			configmap, secret, err = h.generator.Generate(ctx, generateIn)
			if err != nil {
				return microerror.Mask(err)
			}

			h.logger.Debugf(ctx, "generated %#q ConfigMap and Secret for config version %#q", nn, configVersion)
		}
	}

	// Ensure ConfigMap and Secret.
	{
		err = h.resource.EnsureCreated(ctx, meta.Annotation.XObjectHash.Key(), configmap)
		if err != nil {
			return microerror.Mask(err)
		}

		err = h.resource.EnsureCreated(ctx, meta.Annotation.XObjectHash.Key(), secret)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	// Cleanup orphaned ConfigMap and Secret in case previous loop failed in between.
	{
		config, err = h.cleanupOrphanedConfig(ctx, config)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	// Update Config CR status.
	{
		h.logger.Debugf(ctx, "updating Config status")

		desiredStatus := *config.Status.DeepCopy()
		desiredStatus.App = v1alpha1.ConfigStatusApp(config.Spec.App)
		desiredStatus.Config.ConfigMapRef.Name = configmap.Name
		desiredStatus.Config.ConfigMapRef.Namespace = configmap.Namespace
		desiredStatus.Config.SecretRef.Name = secret.Name
		desiredStatus.Config.SecretRef.Namespace = secret.Namespace
		desiredStatus.Version = configVersion

		if reflect.DeepEqual(config.Status, desiredStatus) {
			h.logger.Debugf(ctx, "Config status already up to date")
		} else {
			config.Status = desiredStatus
			err := h.k8sClient.CtrlClient().Status().Update(ctx, config)
			if err != nil {
				return microerror.Mask(err)
			}

			h.logger.Debugf(ctx, "updated Config status")
		}
	}

	// Cleanup orphaned ConfigMap and Secret.
	{
		_, err = h.cleanupOrphanedConfig(ctx, config)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}

func (h *Handler) cleanupOrphanedConfig(ctx context.Context, config *v1alpha1.Config) (*v1alpha1.Config, error) {
	// Get the most recent Config.
	err := h.k8sClient.CtrlClient().Get(ctx, k8sresource.ObjectKey(config), config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	previousConfig, err := meta.Annotation.XPreviousConfig.Get(config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	// If the annotation  is there and the value is equal to the current
	// .status.config so nothing to do here. Return early.
	if reflect.DeepEqual(config.Status.Config, previousConfig) {
		return config, nil
	}

	// Cleanup orphaned config.
	{
		_, orphaned, err := getConfigObjectsMeta(config)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		for _, obj := range orphaned {
			h.logger.Debugf(ctx, "found orphaned %#q %#q", h.resource.Kind(obj), k8sresource.ObjectKey(obj))
		}

		for _, obj := range orphaned {
			err = h.resource.EnsureDeleted(ctx, obj)
			if err != nil {
				return nil, microerror.Mask(err)
			}
		}
	}

	// Now the ConfigMap and the Secret referenced by the annotation (if it
	// exists) are deleted. Update/set the annotation to the current status
	// value.
	{
		h.logger.Debugf(ctx, "updating %#q annotation", meta.Annotation.XPreviousConfig.Key())

		err = meta.Annotation.XPreviousConfig.Set(config, config.Status.Config)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		err = h.k8sClient.CtrlClient().Update(ctx, config)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		h.logger.Debugf(ctx, "updated %#q annotation", meta.Annotation.XPreviousConfig.Key())
	}

	// Try again. If the annotation and the .spec.config value are equal it
	// will return early with an up to date object.
	c, err := h.cleanupOrphanedConfig(ctx, config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return c, nil
}

func (h *Handler) newConfigurationFromDraughtsman(ctx context.Context, in generator.GenerateInput) (*corev1.ConfigMap, *corev1.Secret, error) {
	draughtsmanConfigmap := new(corev1.ConfigMap)
	{
		k := client.ObjectKey{
			Name:      "draughtsman-values-configmap",
			Namespace: "draughtsman",
		}

		err := h.k8sClient.CtrlClient().Get(ctx, k, draughtsmanConfigmap)
		if err != nil {
			return nil, nil, microerror.Mask(err)
		}
	}

	draughtsmanSecret := new(corev1.Secret)
	{
		k := client.ObjectKey{
			Name:      "draughtsman-values-secret",
			Namespace: "draughtsman",
		}

		err := h.k8sClient.CtrlClient().Get(ctx, k, draughtsmanSecret)
		if err != nil {
			return nil, nil, microerror.Mask(err)
		}
	}

	annotations := xstrings.CopyMap(in.ExtraAnnotations)
	annotations[meta.Annotation.ConfigVersion.Key()] = in.ConfigVersion

	meta := metav1.ObjectMeta{
		Name:      in.Name,
		Namespace: in.Namespace,

		Annotations: annotations,
		Labels:      in.ExtraLabels,
	}

	configmap := &corev1.ConfigMap{
		ObjectMeta: meta,
		Data:       draughtsmanConfigmap.Data,
	}

	secret := &corev1.Secret{
		ObjectMeta: meta,
		Data:       draughtsmanSecret.Data,
	}

	return configmap, secret, nil
}

func genStableObjectName(config *v1alpha1.Config) (string, error) {
	h, err := hash(config.Spec.App)
	if err != nil {
		return "", microerror.Mask(err)
	}

	return setSuffixMax63(config.Name, h), nil
}

func hash(v interface{}) (string, error) {
	bs, err := json.Marshal(v)
	if err != nil {
		return "", microerror.Mask(err)
	}

	sum := sha1.Sum(bs) // nolint:gosec
	return fmt.Sprintf("%x", sum)[:10], nil
}

func setSuffixMax63(s string, suffix string) string {
	maxLen := 63

	if len(s)+len(suffix)+1 <= maxLen {
		return s + "-" + suffix
	}

	return s[:maxLen-len(suffix)-1] + "-" + suffix
}
