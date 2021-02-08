package meta

import (
	"github.com/giantswarm/apiextensions/v3/pkg/label"

	"github.com/giantswarm/config-controller/pkg/project"
)

var (
	managedByLabel = label.ManagedBy
)

type ManagedBy struct{}

func (ManagedBy) Key() string { return managedByLabel }

func (ManagedBy) Default() string { return project.Name() }
