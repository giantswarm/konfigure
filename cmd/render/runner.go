package render

import (
	"context"
	"github.com/go-logr/logr"
	"io"

	"github.com/spf13/cobra"
)

const (
	nameSuffix          = "konfigure"
	giantswarmNamespace = "giantswarm"
)

type runner struct {
	flag   *flag
	logger logr.Logger
	stdout io.Writer
	stderr io.Writer
}

func (r *runner) Run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	err := r.flag.Validate()
	if err != nil {
		return err
	}

	err = r.run(ctx, cmd, args)
	if err != nil {
		return err
	}

	return nil
}

func (r *runner) run(ctx context.Context, cmd *cobra.Command, args []string) error {
	r.logger.Info("Rendering...")

	return nil
}
