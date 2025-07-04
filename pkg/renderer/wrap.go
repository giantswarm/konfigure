package renderer

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func WrapIntoConfigMap(data, name, namespace string, annotations, labels map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
			Labels:      labels,
		},
		Data: map[string]string{
			"configmap-values.yaml": sanitizeData(data),
		},
	}
}

func WrapIntoSecret(data, name, namespace string, annotations, labels map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
			Labels:      labels,
		},
		Data: map[string][]byte{
			"secret-values.yaml": []byte(sanitizeData(data)),
		},
	}
}

func sanitizeData(data string) string {
	sanitizedData := data

	if strings.TrimSpace(data) == "null" {
		sanitizedData = ""
	}

	return sanitizedData
}
