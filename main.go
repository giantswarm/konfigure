package main

import (
	"context"
	"fmt"
	"os"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"

	"github.com/giantswarm/konfigure/cmd/generate"
	"github.com/giantswarm/konfigure/cmd/kustomizepatch"
	"github.com/giantswarm/konfigure/cmd/lint"
	"github.com/giantswarm/konfigure/pkg/project"
)

const (
	konfigureModeEnvVar = "KONFIGURE_MODE"
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
		c := kustomizepatch.Config{
			Logger: logger,
		}
		cmd, err := kustomizepatch.New(c)
		if err != nil {
			return microerror.Mask(err)
		}
		subcommands = append(subcommands, cmd)

		// Make kustomizepatch the main command if konfigure is running in
		// container as a kustomize plugin. Kustomize does not know how to call
		// sub-commands. This is discovered by setting KONFIGURE_MODE:
		// "kustomizepatch" environment variable.
		if v := os.Getenv(konfigureModeEnvVar); v == "kustomizepatch" {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			err = cmd.Execute()
			if err != nil {
				return microerror.Mask(err)
			}
			return nil
		}
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
	{
		cmd := &cobra.Command{
			Use:   "version",
			Short: "Display version information.",
			Long:  "Display version information.",
			Run: func(_ *cobra.Command, _ []string) {
				fmt.Println(commandVersion())
			},
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
		"Description: %s\nGitCommit: %s\nName: %s\nSource: %s\nVersion: %s",
		project.Description(),
		project.GitSHA(),
		project.Name(),
		project.Source(),
		project.Version(),
	)
}
