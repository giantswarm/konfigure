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

	//r.logger.Info(fmt.Sprintf("%+v\n", schema))

	r.logger.Info("Loading variables...")
	r.logger.Info("")

	variables, err := renderer.LoadSchemaVariables(r.flag.Variables, schema.Variables)
	if err != nil {
		r.logger.Error(err, "Failed to load variables from flags for schema", "schema", r.flag.Schema)
	}

	//r.logger.Info(fmt.Sprintf("%+v\n", variables))

	r.logger.Info("Loading value files...")

	valueFiles, err := renderer.LoadValueFiles(r.flag.Dir, schema, variables)
	if err != nil {
		r.logger.Error(err, "Failed to load value files")
	}

	//r.logger.Info(fmt.Sprintf("%+v\n", valueFiles))

	r.logger.Info("Loading templates...")
	r.logger.Info("")

	loadedTemplates, err := renderer.LoadTemplates(r.flag.Dir, schema, variables)
	if err != nil {
		r.logger.Error(err, "Failed to load templates")
	}

	//r.logger.Info(fmt.Sprintf("%+v\n", loadedTemplates))

	r.logger.Info("Rendering templates...")
	r.logger.Info("")

	renderedTemplates, err := renderer.RenderTemplates(r.flag.Dir, schema, loadedTemplates, valueFiles)
	if err != nil {
		r.logger.Error(err, "Failed to render templates")
	}

	//r.logger.Info(fmt.Sprintf("%+v\n", renderedTemplates))

	r.logger.Info("Merging rendered templates...")
	r.logger.Info("")

	configmap, secret, err := renderer.MergeRenderedTemplates(schema, renderedTemplates)
	if err != nil {
		r.logger.Error(err, "Failed to merge rendered templates")
	}

	r.logger.Info("# Rendered configmap data:")
	r.logger.Info(fmt.Sprintf("%+v\n", configmap))
	r.logger.Info("")

	r.logger.Info("# Rendered secret data:")
	r.logger.Info(fmt.Sprintf("%+v\n", secret))
	r.logger.Info("")

	return nil
}
