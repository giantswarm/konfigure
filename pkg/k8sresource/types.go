package k8sresource

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Object interface {
	runtime.Object

	GetAnnotations() map[string]string
	GetName() string
	GetNamespace() string
	SetAnnotations(map[string]string)
	GetObjectKind() schema.ObjectKind
}
