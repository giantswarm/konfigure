package service

import (
	"github.com/giantswarm/operatorkit/v4/pkg/flag/service/kubernetes"

	"github.com/giantswarm/config-controller/flag/service/app"
	"github.com/giantswarm/config-controller/flag/service/installation"
)

// Service is an intermediate data structure for command line configuration flags.
type Service struct {
	App          app.App
	Installation installation.Installation
	Kubernetes   kubernetes.Kubernetes
}
