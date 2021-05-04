package generate

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"

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

func createVaultClientUsingOpsctl(ctx context.Context, gitHubToken, installation string) (*vaultapi.Client, error) {
	cmdArgs := []string{"opsctl", "create", "vaultconfig", "-i", installation, "-o", "json"}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) //nolint:gosec
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "OPSCTL_GITHUB_TOKEN="+gitHubToken)
	out, err := cmd.Output()
	if err != nil {
		return nil, microerror.Maskf(
			executionFailedError,
			"failed to execute %#q, see the output above",
			strings.Join(cmdArgs, " "),
		)
	}

	var config vaultClientConfig
	err = json.Unmarshal(out, &config)
	if err != nil {
		return nil, microerror.Maskf(
			executionFailedError,
			"failed to unmarshal output of %#q with error %#q",
			strings.Join(cmdArgs, " "), err,
		)
	}

	vaultClient, err := newVaultClient(config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

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
