package decrypt

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/giantswarm/microerror"
	vaultapi "github.com/hashicorp/vault/api"
)

const (
	path = "/transit/decrypt/config"
)

type VaultDecrypterConfig struct {
	VaultClient *vaultapi.Client
}

type VaultDecrypter struct {
	vaultClient *vaultapi.Client
}

var _ Decrypter = &VaultDecrypter{}

func NewVaultDecrypter(config VaultDecrypterConfig) (*VaultDecrypter, error) {
	if config.VaultClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.VaultClient must not be empty", config)
	}

	d := &VaultDecrypter{
		vaultClient: config.VaultClient,
	}

	return d, nil
}

func (d *VaultDecrypter) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	data, err := d.decrypt(ctx, ciphertext)

	if err != nil {
		return nil, microerror.Mask(err)
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return decoded, nil
}

func (d *VaultDecrypter) decrypt(ctx context.Context, ciphertext []byte) (string, error) {
	secret, err := d.vaultClient.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"ciphertext": string(ciphertext),
	})

	if err != nil {
		return "", microerror.Mask(err)
	}

	return fmt.Sprintf("%v", secret.Data["plaintext"]), nil
}
