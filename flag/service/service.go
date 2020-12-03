package service

import (
	"github.com/giantswarm/operatorkit/v4/pkg/flag/service/kubernetes"

	"github.com/giantswarm/config-controller/flag/service/app"
	"github.com/giantswarm/config-controller/flag/service/github"
	"github.com/giantswarm/config-controller/flag/service/installation"
	"github.com/giantswarm/config-controller/flag/service/vault"
)

// Service is an intermediate data structure for command line configuration flags.
type Service struct {
	App          app.App
	GitHub       github.GitHub
	Installation installation.Installation
	Kubernetes   kubernetes.Kubernetes
	Vault        vault.Vault
}
