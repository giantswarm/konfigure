package generate

import (
	"context"
	"os"

	"github.com/giantswarm/microerror"
	vaultapi "github.com/hashicorp/vault/api"
)

type vaultClientConfig struct {
	Address string `json:"addr"`
	Token   string `json:"token"`
	CAPath  string `json:"caPath"`
}

func newVaultClient(config vaultClientConfig) (*vaultapi.Client, error) {
	c := vaultapi.DefaultConfig()
	c.Address = config.Address
	c.MaxRetries = 4 // Total of 5 tries.
	err := c.ConfigureTLS(&vaultapi.TLSConfig{
		CAPath: config.CAPath,
	})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	vaultClient, err := vaultapi.NewClient(c)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	vaultClient.SetToken(config.Token)

	return vaultClient, nil
}

func createVaultClientUsingEnv(ctx context.Context) (*vaultapi.Client, error) {
	for _, varName := range []string{"VAULT_ADDR", "VAULT_TOKEN", "VAULT_CAPATH"} {
		if value, ok := os.LookupEnv(varName); !ok || value == "" {
			return nil, microerror.Maskf(executionFailedError, "%s environment variable must be set", varName)
		}
	}

	vaultClient, err := newVaultClient(vaultClientConfig{
		Address: os.Getenv("VAULT_ADDR"),
		Token:   os.Getenv("VAULT_TOKEN"),
		CAPath:  os.Getenv("VAULT_CAPATH"),
	})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return vaultClient, nil

}
