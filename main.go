package main

import (
	"context"
	"fmt"
	"os"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/microkit/command"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"

	"github.com/giantswarm/config-controller/cmd/generate"
	"github.com/giantswarm/config-controller/cmd/lint"
	"github.com/giantswarm/config-controller/pkg/project"
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
	var newCommand command.Command
	{
		c := command.Config{
			Logger: logger,

			Description: project.Description(),
			GitCommit:   project.GitSHA(),
			Name:        project.Name(),
			Source:      project.Source(),
			Version:     project.Version(),
		}

		newCommand, err = command.New(c)
		if err != nil {
			return microerror.Mask(err)
		}
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

	newCommand.CobraCommand().SilenceErrors = true
	newCommand.CobraCommand().SilenceUsage = true
	newCommand.CobraCommand().AddCommand(subcommands...)

	err = newCommand.CobraCommand().Execute()
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
