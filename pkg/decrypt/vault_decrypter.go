package decrypt

import (
	"context"
	"encoding/base64"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/valuemodifier/vault/decrypt"
	vaultapi "github.com/hashicorp/vault/api"
)

const (
	key = "config"
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
	service, err := decrypt.New(decrypt.Config{VaultClient: d.vaultClient, Key: key})

	if err != nil {
		return nil, microerror.Mask(err)
	}

	plainText, err := service.Decrypt(ciphertext)

	if err != nil {
		return nil, microerror.Mask(err)
	}

	decoded, err := base64.StdEncoding.DecodeString(plainText)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return decoded, nil
}
