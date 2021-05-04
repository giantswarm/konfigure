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
	"github.com/giantswarm/config-controller/flag"
	"github.com/giantswarm/config-controller/pkg/project"
)

var (
	f *flag.Flag = flag.New()
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
			Logger:        logger,
			ServerFactory: serverFactory,

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

	newCommand.CobraCommand().AddCommand(subcommands...)

	daemonCommand := newCommand.DaemonCommand().CobraCommand()

	daemonCommand.PersistentFlags().Bool(f.Service.App.Unique, false, "Whether the operator is deployed as a unique app.")
	daemonCommand.PersistentFlags().String(f.Service.Installation.Name, "", `Installation codename (e.g. "geckon")`)
	daemonCommand.PersistentFlags().String(f.Service.Kubernetes.Address, "http://127.0.0.1:6443", "Address used to connect to Kubernetes. When empty in-cluster config is created.")
	daemonCommand.PersistentFlags().Bool(f.Service.Kubernetes.InCluster, false, "Whether to use the in-cluster config to authenticate with Kubernetes.")
	daemonCommand.PersistentFlags().String(f.Service.Kubernetes.KubeConfig, "", "KubeConfig used to connect to Kubernetes. When empty other settings are used.")
	daemonCommand.PersistentFlags().String(f.Service.Kubernetes.TLS.CAFile, "", "Certificate authority file path to use to authenticate with Kubernetes.")
	daemonCommand.PersistentFlags().String(f.Service.Kubernetes.TLS.CrtFile, "", "Certificate file path to use to authenticate with Kubernetes.")
	daemonCommand.PersistentFlags().String(f.Service.Kubernetes.TLS.KeyFile, "", "Key file path to use to authenticate with Kubernetes.")

	newCommand.CobraCommand().SilenceErrors = true
	newCommand.CobraCommand().SilenceUsage = true

	err = newCommand.CobraCommand().Execute()
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
