package generate

import (
	"github.com/giantswarm/microerror"
	"github.com/spf13/cobra"
)

const (
	flagApp           = "app"
	flagBranch        = "branch"
	flagInstallation  = "installation"
	flagConfigVersion = "config-version"
)

type flag struct {
	App           string
	Branch        string
	Installation  string
	ConfigVersion string
}

func (f *flag) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.App, flagApp, "", `Name of an application to generate the config for (e.g. "kvm-operator").`)
	cmd.Flags().StringVar(&f.Branch, flagBranch, "", "Branch of giantswarm/config used to generate configuraton.")
	cmd.Flags().StringVar(&f.Installation, flagInstallation, "", `Installation codename (e.g. "gauss").`)
	cmd.Flags().StringVar(&f.ConfigVersion, flagConfigVersion, "", `Version of config to use for generation (e.g. "v2.3.19").`)
}

func (f *flag) Validate() error {
	if f.App == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagApp)
	}
	if f.Installation == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagInstallation)
	}
	if f.ConfigVersion == "" && f.Branch == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagConfigVersion)
	}

	return nil
}
