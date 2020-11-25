package key

import (
	"github.com/giantswarm/config-controller/pkg/project"
)

const (
	Owner                      = "giantswarm"
	KubernetesManagedByLabel   = "app.kubernetes.io/managed-by"
	GiantswarmManagedByLabel   = "giantswarm.io/managed-by"
	ReleaseNameAnnotation      = "meta.helm.sh/release-name"
	ReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
)

var (
	ConfigVersion = project.Name() + ".giantswarm.io/config-version"
)
