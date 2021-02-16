package k8sresource

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func Test_Service_Kind(t *testing.T) {
	s := &Service{
		scheme: scheme.Scheme,
	}

	{
		// Check unregistered.
		type Unregistered struct {
			Object
		}

		o := Unregistered{}

		kind := s.Kind(o)
		if !reflect.DeepEqual(kind, "Unregistered") {
			t.Fatalf("kind = %v, want %v", kind, "Unregistered")
		}
	}

	{
		// Use an alias to make sure scheme is used.
		type CMAlias = corev1.ConfigMap

		o := &CMAlias{}
		kind := s.Kind(o)
		if !reflect.DeepEqual(kind, "ConfigMap") {
			t.Fatalf("kind = %v, want %v", kind, "ConfigMap")
		}
	}
}
