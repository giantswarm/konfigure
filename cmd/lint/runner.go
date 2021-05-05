package lint

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"

	"github.com/giantswarm/konfigure/pkg/filesystem"
	"github.com/giantswarm/konfigure/pkg/lint"
)

type runner struct {
	flag   *flag
	logger micrologger.Logger
	stdout io.Writer
	stderr io.Writer
}

func (r *runner) Run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	err := r.flag.Validate()
	if err != nil {
		return microerror.Mask(err)
	}

	err = r.run(ctx, cmd, args)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (r *runner) run(ctx context.Context, cmd *cobra.Command, args []string) error {
	var linter *lint.Linter
	{
		skipFieldsREs := strings.Split(r.flag.SkipFieldsRegexp, ",")

		c := lint.Config{
			Store:            &filesystem.Store{},
			FilterFunctions:  r.flag.FilterFunctions,
			OnlyErrors:       r.flag.OnlyErrors,
			MaxMessages:      r.flag.MaxMessages,
			SkipFieldsRegexp: skipFieldsREs,
		}

		l, err := lint.New(c)
		if err != nil {
			return microerror.Mask(err)
		}
		linter = l
	}

	messages := linter.Lint(ctx)

	for _, msg := range messages {
		fmt.Println(msg.Message(!r.flag.NoFuncNames, !r.flag.NoDescriptions))
	}

	if r.flag.MaxMessages > 0 && len(messages) == r.flag.MaxMessages {
		fmt.Println("-------------------------")
		fmt.Println("Too many messages, skipping the rest of checks")
		fmt.Printf("Run linter with '--%s 0' to see all the errors\n", flagMaxMessages)
		return microerror.Mask(linterFoundIssuesError)
	}

	fmt.Printf("-------------------------\nFound %d issues\n", len(messages))
	if len(messages) > 0 {
		return microerror.Mask(linterFoundIssuesError)
	}

	return nil
}
