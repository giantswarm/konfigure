package generate

import (
	"github.com/giantswarm/microerror"
	"github.com/spf13/cobra"
)

const (
	flagApp        = "app" // TODO: app-name
	flagAppCatalog = "app-catalog"
	// flagAppCRName              = "appcr-name" // TODO: is this even needed
	flagAppDisableForceUpgrade = "app-disable-force-upgrade"
	flagAppVersion             = "app-version"
	flagDir                    = "dir"
	flagInstallation           = "installation"
	flagName                   = "name" // TODO: config-name
	flagNamespace              = "namespace"
	flagRaw                    = "raw"
	flagVerbose                = "verbose"
)

type flag struct {
	App        string
	AppCatalog string
	// AppCRName              string
	AppDisableForceUpgrade bool
	AppVersion             string
	Dir                    string
	Installation           string
	Namespace              string
	Name                   string
	Raw                    bool
	Verbose                bool
}

func (f *flag) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.App, flagApp, "", `Name of an application to generate the config for (e.g. "kvm-operator").`)
	cmd.Flags().StringVar(&f.AppCatalog, flagAppCatalog, "", `Name of application catalog, e.g. "control-plane-test-catalog".`)
	cmd.Flags().BoolVar(&f.AppDisableForceUpgrade, flagAppDisableForceUpgrade, false, `Sets "chart-operator.giantswarm.io/force-helm-upgrade" flag.`)
	cmd.Flags().StringVar(&f.AppVersion, flagAppVersion, "", `Application version to be set in App CR.`)
	cmd.Flags().StringVar(&f.Dir, flagDir, ".", `Directory containing configuration source (e.g cloned "giantswarm/config" repo).`)
	cmd.Flags().StringVar(&f.Installation, flagInstallation, "", `Installation codename (e.g. "gauss").`)
	cmd.Flags().StringVar(&f.Name, flagName, "giantswarm", `Name of the generated ConfigMap/Secret.`)
	cmd.Flags().StringVar(&f.Namespace, flagNamespace, "giantswarm", `Namespace of the generated ConfigMap/Secret.`)
	cmd.Flags().BoolVar(&f.Raw, flagRaw, false, `Forces generator to output YAML instead of ConfigMap & Secret.`)
	cmd.Flags().BoolVar(&f.Verbose, flagVerbose, false, `Enables generator to output consecutive generation stages.`)
}

func (f *flag) Validate() error {
	if f.App == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagApp)
	}
	if f.AppCatalog == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagAppCatalog)
	}
	if f.AppVersion == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagAppVersion)
	}
	if f.Dir == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagDir)
	}
	if f.Installation == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagInstallation)
	}
	if f.Name == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagName)
	}
	if f.Namespace == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagNamespace)
	}

	return nil
}
