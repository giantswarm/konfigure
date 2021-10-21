package meta

import (
	"github.com/giantswarm/apiextensions/v3/pkg/label"
	"github.com/giantswarm/operatorkit/v5/pkg/controller"

	"github.com/giantswarm/konfigure/pkg/project"
)

var (
	managedByLabel = label.ManagedBy
	versionLabel   = label.ConfigControllerVersion
)

type ManagedBy struct{}

func (ManagedBy) Key() string { return managedByLabel }

func (ManagedBy) Default() string { return project.Name() }

type Version struct{}

func (Version) Key() string { return versionLabel }

func (Version) Val(uniqueApp bool) string {
	if uniqueApp {
		// When konfigure is deployed as a unique app it only
		// processes management cluster CRs. These CRs always have the
		// version label konfigure.giantswarm.io/version: 0.0.0
		return "0.0.0"
	} else {
		return project.Version()
	}
}

func (Version) Selector(uniqueApp bool) controller.Selector {
	return controller.NewSelector(func(labels controller.Labels) bool {
		if !labels.Has(versionLabel) {
			return false
		}
		if labels.Get(versionLabel) == (Version{}).Val(uniqueApp) {
			return true
		}

		return false
	})
}
