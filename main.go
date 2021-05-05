package main

import (
	"context"
	"fmt"
	"os"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"

	"github.com/giantswarm/konfigure/cmd/generate"
	"github.com/giantswarm/konfigure/cmd/lint"
	"github.com/giantswarm/konfigure/pkg/project"
)

func main() {
	err := mainE(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", microerror.Pretty(microerror.Mask(err), true))
		os.Exit(1)
	}
}

func mainE(ctx context.Context) error {
	var err error

	var logger micrologger.Logger
	{
		c := micrologger.Config{}

		logger, err = micrologger.New(c)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	// Create a new microkit command which manages our custom microservice.
	newCommand := &cobra.Command{
		Use:     project.Name(),
		Long:    project.Description(),
		Version: commandVersion(),
	}

	// Add sub-commands
	subcommands := []*cobra.Command{}
	{
		c := generate.Config{
			Logger: logger,
		}
		cmd, err := generate.New(c)
		if err != nil {
			return microerror.Mask(err)
		}
		subcommands = append(subcommands, cmd)
	}
	{
		c := lint.Config{
			Logger: logger,
		}
		cmd, err := lint.New(c)
		if err != nil {
			return microerror.Mask(err)
		}
		subcommands = append(subcommands, cmd)
	}

	newCommand.SilenceErrors = true
	newCommand.SilenceUsage = true
	newCommand.AddCommand(subcommands...)

	err = newCommand.Execute()
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func commandVersion() string {
	return fmt.Sprintf(
		"\nDescription: %s\nGitCommit: %s\nName: %s\nSource: %s\nVersion: %s",
		project.Description(),
		project.GitSHA(),
		project.Name(),
		project.Source(),
		project.Version(),
	)
}
