package key

const (
	Owner                      = "giantswarm"
	KubernetesManagedByLabel   = "app.kubernetes.io/managed-by"
	ReleaseNameAnnotation      = "meta.helm.sh/release-name"
	ReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
	ArgoResourceFinalizer      = "resources-finalizer.argocd.argoproj.io"
)
