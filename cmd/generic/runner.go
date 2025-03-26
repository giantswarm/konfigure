package generic

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
)

type runner struct {
	flag   *flag
	logger logr.Logger
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
	fmt.Println("Hello world!")

	content, err := os.ReadFile(r.flag.Schema)
	if err != nil {
		return err
	}

	var schema Schema
	err = yaml.Unmarshal(content, &schema)
	if err != nil {
		return err
	}

	for _, layer := range schema.Spec.Layers {
		fmt.Println(layer.Name)
	}

	return nil
}
