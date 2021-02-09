package meta

import (
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/operatorkit/v4/pkg/controller"

	"github.com/giantswarm/config-controller/pkg/project"
)

var (
	managedByLabel = label.ManagedBy
	version        = label.ConfigControllerVersion
)

type ManagedBy struct{}

func (ManagedBy) Key() string { return managedByLabel }

func (ManagedBy) Default() string { return project.Name() }

type Version struct{}

func (Version) Key() string { return version }

func (Version) Val(uniqueApp bool) string {
	if uniqueApp {
		// When config-controller is deployed as a unique app it only
		// processes management cluster CRs. These CRs always have the
		// version label config-controller.giantswarm.io/version: 0.0.0
		return "0.0.0"
	} else {
		return project.Version()
	}
}

func (Version) Selector(uniqueApp bool) controller.Selector {
	return controller.NewSelector(func(labels controller.Labels) bool {
		if !labels.Has(version) {
			return false
		}
		if labels.Get(version) == (Version{}).Val(uniqueApp) {
			return true
		}

		return false
	})
}
