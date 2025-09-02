package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"

	"github.com/spf13/cobra"

	"github.com/giantswarm/konfigure/v2/cmd/render"
	"github.com/giantswarm/konfigure/v2/pkg/project"
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

	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}

	logger := zapr.NewLogger(zapLogger)

	newCommand := &cobra.Command{
		Use:     project.Name(),
		Long:    project.Description(),
		Version: commandVersion(),
	}

	// Add sub-commands
	subcommands := []*cobra.Command{}
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
	{
		c := render.Config{
			Logger: logger,
		}
		cmd, err := render.New(c)
		if err != nil {
			return err
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
