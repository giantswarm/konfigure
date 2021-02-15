package lint

import (
	"context"
	"fmt"
	"io"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"

	"github.com/giantswarm/config-controller/pkg/generator"
	"github.com/giantswarm/config-controller/pkg/github"
	"github.com/giantswarm/config-controller/pkg/lint"
)

const (
	owner = "giantswarm"
	repo  = "config"
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
	var store generator.Filesystem
	{
		gh, err := github.New(github.Config{
			Token: r.flag.GitHubToken,
		})
		if err != nil {
			return microerror.Mask(err)
		}

		if r.flag.ConfigVersion != "" {
			tag, err := gh.GetLatestTag(ctx, owner, repo, r.flag.ConfigVersion)
			if err != nil {
				return microerror.Mask(err)
			}

			store, err = gh.GetFilesByTag(ctx, owner, repo, tag)
			if err != nil {
				return microerror.Mask(err)
			}

		} else if r.flag.Branch != "" {
			store, err = gh.GetFilesByBranch(ctx, owner, repo, r.flag.Branch)
			if err != nil {
				return microerror.Mask(err)
			}
		}
	}

	var linter *lint.Linter
	{
		c := lint.Config{
			Store:           store,
			FilterFunctions: r.flag.FilterFunctions,
			OnlyErrors:      r.flag.OnlyErrors,
			MaxMessages:     r.flag.MaxMessages,
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
