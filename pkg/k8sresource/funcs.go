package k8sresource

import "k8s.io/apimachinery/pkg/types"

func Kind(o Object) string {
	return o.GetObjectKind().GroupVersionKind().Kind
}

func NamespacedName(o Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: o.GetNamespace(),
		Name:      o.GetName(),
	}
}
