package generate

import (
	"context"
	"fmt"
	"io"

	"github.com/ghodss/yaml"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"

	"github.com/giantswarm/config-controller/pkg/decrypt"
	"github.com/giantswarm/config-controller/pkg/generator"
	"github.com/giantswarm/config-controller/pkg/github"
)

const (
	owner = "giantswarm"
	repo  = "config"
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
	var decryptTraverser *decrypt.YAMLTraverser
	{
		vaultClient, err := createVaultClientUsingOpsctl(ctx, r.flag.GitHubToken, r.flag.Installation)
		if err != nil {
			return microerror.Mask(err)
		}

		c := decrypt.VaultDecrypterConfig{
			VaultClient: vaultClient,
		}

		decrypter, err := decrypt.NewVaultDecrypter(c)
		if err != nil {
			return microerror.Mask(err)
		}

		decryptTraverser, err = decrypt.NewYAMLTraverser(
			decrypt.YAMLTraverserConfig{
				Decrypter: decrypter,
			},
		)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	var store generator.Filesystem
	var ref string
	{
		gh, err := github.New(github.Config{
			Token: r.flag.GitHubToken,
		})
		if err != nil {
			return microerror.Mask(err)
		}

		if r.flag.ConfigVersion != "" {
			tag, err := gh.GetLatestTag(ctx, owner, repo, r.flag.ConfigVersion)
			if err != nil {
				return microerror.Mask(err)
			}

			store, err = gh.GetFilesByTag(ctx, owner, repo, tag)
			if err != nil {
				return microerror.Mask(err)
			}

			ref = tag
		} else if r.flag.Branch != "" {
			store, err = gh.GetFilesByBranch(ctx, owner, repo, r.flag.Branch)
			if err != nil {
				return microerror.Mask(err)
			}

			ref = r.flag.Branch
		}
	}

	gen, err := generator.New(&generator.Config{
		Fs:               store,
		DecryptTraverser: decryptTraverser,
	})
	if err != nil {
		return microerror.Mask(err)
	}

	configmap, secrets, err := gen.GenerateConfig(ctx, r.flag.Installation, r.flag.App, ref)
	if err != nil {
		return microerror.Mask(err)
	}

	fmt.Println("---")
	out, err := yaml.Marshal(configmap)
	if err != nil {
		return microerror.Mask(err)
	}
	fmt.Printf(string(out) + "\n")

	fmt.Println("---")
	out, err = yaml.Marshal(secrets)
	if err != nil {
		return microerror.Mask(err)
	}
	fmt.Printf(string(out) + "\n")

	return nil
}
