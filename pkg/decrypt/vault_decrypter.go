package decrypt

import (
	"context"
	"encoding/base64"
	"github.com/giantswarm/konfigure/internal/vaultclient"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/valuemodifier/vault/decrypt"
)

const (
	key = "config"
)

type VaultDecrypterConfig struct {
	VaultClient *vaultclient.WrappedVaultClient
}

type VaultDecrypter struct {
	vaultClient *vaultclient.WrappedVaultClient
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
	if err := d.vaultClient.ConfigurationValidator(); err != nil {
		return nil, microerror.Mask(err)
	}

	service, err := decrypt.New(decrypt.Config{VaultClient: d.vaultClient.Wrapped, Key: key})

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
