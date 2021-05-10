package generate

import (
	"github.com/giantswarm/microerror"
	"github.com/spf13/cobra"
)

const (
	flagAppCatalog             = "app-catalog"
	flagAppDisableForceUpgrade = "app-disable-force-upgrade"
	flagAppName                = "app-name"
	flagAppVersion             = "app-version"
	flagDir                    = "dir"
	flagInstallation           = "installation"
	flagName                   = "name"
	flagNamespace              = "namespace"
	flagRaw                    = "raw"
	flagVaultSecretName        = "vault-secret-name"
	flagVaultSecretNamespace   = "vault-secret-namespace"
	flagVerbose                = "verbose"
)

type flag struct {
	AppCatalog             string
	AppDisableForceUpgrade bool
	AppName                string
	AppVersion             string
	Dir                    string
	Installation           string
	Namespace              string
	Name                   string
	Raw                    bool
	VaultSecretName        string
	VaultSecretNamespace   string
	Verbose                bool
}

func (f *flag) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.AppCatalog, flagAppCatalog, "", `Name of application catalog, e.g. "control-plane-test-catalog".`)
	cmd.Flags().BoolVar(&f.AppDisableForceUpgrade, flagAppDisableForceUpgrade, false, `Sets "chart-operator.giantswarm.io/force-helm-upgrade" flag.`)
	cmd.Flags().StringVar(&f.AppName, flagAppName, "", `Name of an application to generate the config for (e.g. "kvm-operator").`)
	cmd.Flags().StringVar(&f.AppVersion, flagAppVersion, "", `Application version to be set in App CR.`)
	cmd.Flags().StringVar(&f.Dir, flagDir, ".", `Directory containing configuration source (e.g cloned "giantswarm/config" repo).`)
	cmd.Flags().StringVar(&f.Installation, flagInstallation, "", `Installation codename (e.g. "gauss").`)
	cmd.Flags().StringVar(&f.Name, flagName, "giantswarm", `Name of the generated ConfigMap/Secret/App.`)
	cmd.Flags().StringVar(&f.Namespace, flagNamespace, "giantswarm", `Namespace of the generated ConfigMap/Secret.`)
	cmd.Flags().BoolVar(&f.Raw, flagRaw, false, `Forces generator to output YAML instead of ConfigMap & Secret.`)
	cmd.Flags().StringVar(&f.VaultSecretName, flagVaultSecretName, "", "Name of K8s secret containing vault credentials (optional).")
	cmd.Flags().StringVar(&f.VaultSecretNamespace, flagVaultSecretNamespace, "", "Namespace of K8s secret containing vault credentials (optional).")
	cmd.Flags().BoolVar(&f.Verbose, flagVerbose, false, `Enables generator to output consecutive generation stages.`)
}

func (f *flag) Validate() error {
	if f.AppName == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagAppName)
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
	if (f.VaultSecretName == "" || f.VaultSecretNamespace == "") && f.VaultSecretName != f.VaultSecretNamespace {
		return microerror.Maskf(invalidFlagError, "you have to specify both or neither %q and %q", flagVaultSecretName, flagVaultSecretNamespace)
	}

	return nil
}
