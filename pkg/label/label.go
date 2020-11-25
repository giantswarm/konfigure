package label

import (
	"github.com/giantswarm/operatorkit/v4/pkg/controller"

	"github.com/giantswarm/config-controller/pkg/project"
)

const (
	version = "config-controller.giantswarm.io/version"
)

func AppVersionSelector(unique bool) controller.Selector {
	return controller.NewSelector(func(labels controller.Labels) bool {
		if !labels.Has(version) {
			return false
		}
		if labels.Get(version) == getProjectVersion(unique) {
			return true
		}

		return false
	})
}

func getProjectVersion(unique bool) string {
	if unique {
		// When config-controller is deployed as a unique app it only
		// processes control plane app CRs. These CRs always have the
		// version label config-controller.giantswarm.io/version: 0.0.0
		return project.AppControlPlaneVersion()
	} else {
		return project.Version()
	}
}
