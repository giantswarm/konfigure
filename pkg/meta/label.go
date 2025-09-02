package meta

import (
	"github.com/giantswarm/k8smetadata/pkg/label"

	"github.com/giantswarm/konfigure/v2/pkg/project"
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
