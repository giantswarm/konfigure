package decrypter

import (
	"context"
	"encoding/base64"

	"github.com/giantswarm/microerror"
	vaultapi "github.com/hashicorp/vault/api"
)

const (
	keyring = "/v1/transit/decrypt/config"
)

type Config struct {
	VaultClient *vaultapi.Client
}

type Decrypter struct {
	vaultClient *vaultapi.Client
}

func New(config Config) (*Decrypter, error) {
	if config.VaultClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.VaultClient must not be empty", config)
	}

	d := &Decrypter{
		vaultClient: config.VaultClient,
	}

	return d, nil
}

func (d *Decrypter) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
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

func (d *Decrypter) vaultRequest(ctx context.Context, endpoint string, req, resp interface{}) error {
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
