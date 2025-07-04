package render

import (
	"context"
	"io"

	"github.com/giantswarm/konfigure/pkg/service"
	"github.com/giantswarm/konfigure/pkg/utils"

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
	dynamicService := service.NewDynamicService(service.DynamicServiceConfig{
		Log: r.logger,
	})

	configMap, secret, err := dynamicService.Render(service.RenderInput{
		// Root directory of the config repository.
		Dir:       r.flag.Dir,
		Schema:    r.flag.Schema,
		Variables: r.flag.Variables,
		Name:      "example",
		Namespace: "default",
	})
	if err != nil {
		return err
	}

	err = utils.PrettyPrint(configMap)
	if err != nil {
		return err
	}

	err = utils.PrettyPrint(secret)
	if err != nil {
		return err
	}

	return nil
}
