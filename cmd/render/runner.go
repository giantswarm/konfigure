package render

import (
	"context"
	"fmt"
	"io"

	"github.com/giantswarm/konfigure/pkg/renderer"

	"github.com/go-logr/logr"

	"github.com/spf13/cobra"
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
	r.logger.Info("")

	schema, err := renderer.LoadSchema(r.flag.Schema)
	if err != nil {
		r.logger.Error(err, "Failed to load schema", "file", r.flag.Schema)
	}

	r.logger.Info(fmt.Sprintf("%+v\n", schema))

	r.logger.Info("Loading variables...")
	r.logger.Info("")

	variables, err := renderer.LoadSchemaVariables(r.flag.Variables, schema.Variables)
	if err != nil {
		r.logger.Error(err, "Failed to load variables from flags for schema", "schema", r.flag.Schema)
	}

	r.logger.Info(fmt.Sprintf("%+v\n", variables))

	return nil
}
