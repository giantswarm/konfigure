package fetchkeys

import (
	"context"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"

	"github.com/giantswarm/konfigure/pkg/sopsenv"
	"github.com/giantswarm/konfigure/pkg/sopsenv/key"
)

type runner struct {
	flag   *flag
	logger logr.Logger
	stdout io.Writer
	stderr io.Writer
}

func (r *runner) Run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	err := r.flag.Validate()
	if err != nil {
		return err
	}

	err = r.run(ctx, cmd, args)
	if err != nil {
		return err
	}

	return nil
}

func (r *runner) run(ctx context.Context, cmd *cobra.Command, args []string) error {
	cfg := sopsenv.SOPSEnvConfig{
		KeysDir:    r.flag.SOPSKeysDir,
		KeysSource: key.KeysSourceKubernetes,
		Logger:     r.logger,
	}

	sopsEnv, err := sopsenv.NewSOPSEnv(cfg)
	if err != nil {
		return err
	}

	err = sopsEnv.Setup(ctx)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(r.stdout, "Keychains Directory: %s\n", sopsEnv.GetKeysDir())

	return nil
}
