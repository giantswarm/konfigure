package main

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"os"

	"github.com/spf13/cobra"

	"github.com/giantswarm/konfigure/cmd/fetchkeys"
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
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}

func mainE(ctx context.Context) error {
	var err error

	logger, err := logr.FromContext(ctx)
	if err != nil {
		return err
	}

	newCommand := &cobra.Command{
		Use:     project.Name(),
		Long:    project.Description(),
		Version: commandVersion(),
	}

	// Add sub-commands
	subcommands := []*cobra.Command{}
	{
		c := fetchkeys.Config{
			Logger: logger,
		}
		cmd, err := fetchkeys.New(c)
		if err != nil {
			return err
		}
		subcommands = append(subcommands, cmd)
	}
	{
		c := generate.Config{
			Logger: logger,
		}
		cmd, err := generate.New(c)
		if err != nil {
			return err
		}
		subcommands = append(subcommands, cmd)
	}
	{
		c := kustomizepatch.Config{
			Logger: logger,
		}
		cmd, err := kustomizepatch.New(c)
		if err != nil {
			return err
		}
		subcommands = append(subcommands, cmd)

		// Make kustomizepatch the main command if konfigure is running in
		// container as a kustomize plugin. Kustomize does not know how to call
		// sub-commands. This is enabled by setting KONFIGURE_MODE:
		// "kustomizepatch" environment variable.
		if v := os.Getenv(konfigureModeEnvVar); v == "kustomizepatch" {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			err = cmd.Execute()
			if err != nil {
				_, err := fmt.Fprint(os.Stderr, err)
				if err != nil {
					return err
				}
				os.Exit(1)
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
			return err
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
		return err
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
