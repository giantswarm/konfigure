package generate

import (
	"context"
	"fmt"
	"io"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"

	"github.com/giantswarm/config-controller/pkg/decrypt"
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
	fmt.Fprintf(r.stdout, "Creating vault client using opsctl\n")

	vaultClient, err := createVaultClientUsingOpsctl(ctx, r.flag.GitHubToken, r.flag.Installation)
	if err != nil {
		return microerror.Mask(err)
	}

	var decrypter *decrypt.Decrypter
	{
		c := decrypt.DecrypterConfig{
			VaultClient: vaultClient,
		}

		decrypter, err = decrypt.New(c)
		if err != nil {
			return microerror.Mask(err)
		}

	}

	if len(args) != 1 {
		fmt.Fprintf(r.stderr, "Error: Expected the first argument to encrypted blob")
	}

	decrypted, err := decrypter.Decrypt(ctx, []byte(args[0]))
	if err != nil {
		return microerror.Mask(err)
	}

	fmt.Fprintf(r.stdout, "Decrypted: %s\n", decrypted)

	return nil
}
