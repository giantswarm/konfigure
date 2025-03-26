package generic

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const (
	flagSchema = "schema"
)

type flag struct {
	Schema string
}

func (f *flag) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.Schema, flagSchema, "config-schema.yaml", `Directory containing configuration source (e.g cloned "giantswarm/config" repo).`)
}

func (f *flag) Validate() error {
	if _, err := os.Stat(f.Schema); errors.Is(err, os.ErrNotExist) {
		return &InvalidFlagError{message: fmt.Sprintf("Schema file not found: %s", f.Schema)}
	}

	return nil
}
