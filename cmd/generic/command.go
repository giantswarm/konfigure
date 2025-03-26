package generic

import (
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
)

const (
	name        = "generic"
	description = "Generate generic configuration based on the provided schema"
)

type Config struct {
	Logger logr.Logger
}

func New(config Config) (*cobra.Command, error) {
	f := &flag{}

	r := &runner{
		flag: f,
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
