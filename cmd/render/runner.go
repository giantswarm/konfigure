package render

import (
	"context"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/giantswarm/konfigure/pkg/model"

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

	f, err := os.ReadFile(r.flag.Schema)
	if err != nil {
		r.logger.Error(err, "Failed to read schema", "file", r.flag.Schema)
	}

	var schema model.Schema
	if err := yaml.Unmarshal(f, &schema); err != nil {
		r.logger.Error(err, "Failed to unmarshal schema", "file", r.flag.Schema)
	}

	r.logger.Info(fmt.Sprintf("%+v\n", schema))

	return nil
}
