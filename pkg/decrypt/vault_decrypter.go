package decrypt

import (
	"context"
	"encoding/base64"

	"github.com/giantswarm/microerror"
	vaultapi "github.com/hashicorp/vault/api"
)

const (
	keyring = "/v1/transit/decrypt/config"
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
	req := struct {
		Ciphertext string `json:"ciphertext"`
	}{
		Ciphertext: string(ciphertext),
	}

	resp := struct {
		Data struct {
			Plaintext string `json:"plaintext"`
		} `json:"data"`
	}{}

	err := d.vaultRequest(ctx, keyring, req, &resp)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.Data.Plaintext)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return decoded, nil
}

func (d *VaultDecrypter) vaultRequest(ctx context.Context, endpoint string, req, resp interface{}) error {
	httpReq := d.vaultClient.NewRequest("POST", endpoint)
	err := httpReq.SetJSONBody(req)
	if err != nil {
		return microerror.Mask(err)
	}

	httpResp, err := d.vaultClient.RawRequest(httpReq)
	if err != nil {
		return microerror.Mask(err)
	}

	if httpResp.StatusCode != 200 {
		return microerror.Maskf(executionFailedError, "expected status code = 200, got %d", httpResp.StatusCode)
	}

	err = httpResp.DecodeJSON(resp)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
