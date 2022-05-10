package fetchkeys

import (
	"github.com/spf13/cobra"
)

const (
	flagSOPSKeysDir = "sops-keys-dir"
)

type flag struct {
	SOPSKeysDir string
}

func (f *flag) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.SOPSKeysDir, flagSOPSKeysDir, "", `Directory to fetch SOPS private keys into (optional)."`)
}

func (f *flag) Validate() error {
	return nil
}
