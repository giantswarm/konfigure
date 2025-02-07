package decrypt

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/giantswarm/konfigure/pkg/vaultclient"

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
		return nil, &InvalidConfigError{message: fmt.Sprintf("%T.VaultClient must not be empty", config)}
	}

	d := &VaultDecrypter{
		vaultClient: config.VaultClient,
	}

	return d, nil
}

func (d *VaultDecrypter) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	if err := d.vaultClient.ConfigurationValidator(); err != nil {
		return nil, err
	}

	service, err := decrypt.New(decrypt.Config{VaultClient: d.vaultClient.Wrapped, Key: key})

	if err != nil {
		return nil, err
	}

	plainText, err := service.Decrypt(ciphertext)

	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(plainText)
	if err != nil {
		return nil, err
	}

	return decoded, nil
}
