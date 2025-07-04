package render

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/giantswarm/konfigure/pkg/sopsenv/key"
)

const (
	flagSchema         = "schema"
	flagDir            = "dir"
	flagInstallation   = "installation"
	flagName           = "name"
	flagRaw            = "raw"
	flagSOPSKeysSource = "sops-keys-source"
	flagSOPSKeysDir    = "sops-keys-dir"
	flagVerbose        = "verbose"
	flagVariable       = "variable"
)

type flag struct {
	Schema         string
	Dir            string
	Raw            bool
	SOPSKeysDir    string
	SOPSKeysSource string
	Verbose        bool
	Variables      []string
}

func (f *flag) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.Schema, flagSchema, "", `Path to the schema file.`)
	cmd.Flags().StringVar(&f.Dir, flagDir, ".", `Directory containing configuration source (e.g cloned "giantswarm/config" repo).`)
	cmd.Flags().BoolVar(&f.Raw, flagRaw, false, `Forces generator to output YAML instead of ConfigMap & Secret.`)
	cmd.Flags().StringVar(&f.SOPSKeysDir, flagSOPSKeysDir, "", `Directory containing SOPS private keys (optional).`)
	cmd.Flags().StringVar(&f.SOPSKeysSource, flagSOPSKeysSource, "local", `Source of SOPS private keys, supports "local" and "kubernetes", (optional).`)
	cmd.Flags().BoolVar(&f.Verbose, flagVerbose, false, `Enables generator to output consecutive generation stages.`)
	cmd.Flags().StringArrayVar(&f.Variables, flagVariable, []string{}, `Variables for rendering the schema.`)
}

func (f *flag) Validate() error {
	if f.Schema == "" {
		return &InvalidFlagError{message: fmt.Sprintf("--%s must not be empty", flagSchema)}
	}
	if f.Dir == "" {
		return &InvalidFlagError{message: fmt.Sprintf("--%s must not be empty", flagDir)}
	}
	if f.SOPSKeysSource != key.KeysSourceLocal && f.SOPSKeysSource != key.KeysSourceKubernetes {
		return &InvalidFlagError{message: fmt.Sprintf("--%s must be one of: %s", flagSOPSKeysSource, "local,kubernetes")}
	}

	return nil
}
