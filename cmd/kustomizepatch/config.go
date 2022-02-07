package kustomizepatch

import "github.com/giantswarm/microerror"

// config is required configuration passed by the generator
// Example:
// ```
// apiVersion: generators.giantswarm.io/v1
// kind: Konfigure
// metadata:
//   name: konfigure-plugin
//   annotations:
//     config.kubernetes.io/function: |
//       exec:
//         path: /plugins/konfigure
// app_catalog: ""
// app_destination_namespace: ""
// app_disable_force_upgrade: true
// app_name: ""
// app_version: ""
// name: ""
// ```
type config struct {
	// AppCatalog is a name of application catalog, e.g. "control-plane-test-catalog".
	AppCatalog string `json:"app_catalog" yaml:"app_catalog"`
	// AppDestinationNamespace sets the namespace where application resources will be installed.
	AppDestinationNamespace string `json:"app_destination_namespace" yaml:"app_destination_namespace"`
	// AppDisableForceUpgrade sets "chart-operator.giantswarm.io/force-helm-upgrade" flag.
	AppDisableForceUpgrade bool `json:"app_disable_force_upgrade" yaml:"app_disable_force_upgrade"`
	// AppName is a name of an application to generate the config for (e.g. "kvm-operator").
	AppName string `json:"app_name" yaml:"app_name"`
	// AppVersion is application version to be set in App CR.
	AppVersion string `json:"app_version" yaml:"app_version"`
	// Name is the name of the generated ConfigMap/Secret/App.
	Name string `json:"name" yaml:"name"`
}

func (c *config) Validate() error {
	if c.AppCatalog == "" {
		return microerror.Maskf(invalidConfigError, "%T.AppCatalog is required", c)
	}
	if c.AppDestinationNamespace == "" {
		return microerror.Maskf(invalidConfigError, "%T.AppDestinationNamespace is required", c)
	}
	if c.AppName == "" {
		return microerror.Maskf(invalidConfigError, "%T.AppName is required", c)
	}
	if c.AppVersion == "" {
		return microerror.Maskf(invalidConfigError, "%T.AppVersion is required", c)
	}
	if c.Name == "" {
		return microerror.Maskf(invalidConfigError, "%T.Name is required", c)
	}
	return nil
}
