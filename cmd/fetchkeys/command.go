package fetchkeys

import (
	"io"
	"os"

	"github.com/go-logr/logr"

	"github.com/spf13/cobra"
)

const (
	name        = "fetchkeys"
	description = "Fetch SOPS keys from Kubernetes Secrets."
)

type Config struct {
	Logger logr.Logger
	Stderr io.Writer
	Stdout io.Writer
}

func New(config Config) (*cobra.Command, error) {
	if config.Stderr == nil {
		config.Stderr = os.Stderr
	}
	if config.Stdout == nil {
		config.Stdout = os.Stdout
	}

	f := &flag{}

	r := &runner{
		flag:   f,
		logger: config.Logger,
		stderr: config.Stderr,
		stdout: config.Stdout,
	}

	c := &cobra.Command{
		Use:   name,
		Short: description,
		Long:  description,
		RunE:  r.Run,
	}

	f.Init(c)

	return c, nil
}
