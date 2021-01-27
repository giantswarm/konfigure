package k8sresource

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DeleteAnnotation(o Object, key string) {
	a := o.GetAnnotations()
	if a == nil {
		return
	}

	delete(a, key)
	o.SetAnnotations(a)
}

func GetAnnotation(o Object, key string) (string, bool) {
	a := o.GetAnnotations()
	if a == nil {
		return "", false
	}

	s, ok := a[key]
	return s, ok
}

func SetAnnotation(o Object, key, val string) {
	a := o.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}

	a[key] = val
	o.SetAnnotations(a)
}

func ObjectKey(o Object) client.ObjectKey {
	return client.ObjectKey{
		Namespace: o.GetNamespace(),
		Name:      o.GetName(),
	}
}
