package vaultclient

import (
	"context"
	"os"

	"github.com/giantswarm/microerror"
	vaultapi "github.com/hashicorp/vault/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	vaultAddress = "VAULT_ADDR"
	vaultToken   = "VAULT_TOKEN"
	vaultCAPath  = "VAULT_CAPATH"
)

type Config struct {
	Address string `json:"addr"`
	Token   string `json:"token"`
	CAPath  string `json:"caPath"`
}

func NewClient(config Config) (*vaultapi.Client, error) {
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

func NewClientUsingK8sSecret(ctx context.Context, namespace, name string) (*vaultapi.Client, error) {
	c, err := rest.InClusterConfig()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	client, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	for _, varName := range []string{vaultAddress, vaultToken, vaultCAPath} {
		if value, ok := secret.Data[varName]; !ok || string(value) == "" {
			return nil, microerror.Maskf(executionFailedError, "secret.Data must contain %q", varName)
		}
	}

	vaultClient, err := NewClient(Config{
		Address: string(secret.Data[vaultAddress]),
		Token:   string(secret.Data[vaultToken]),
		CAPath:  string(secret.Data[vaultCAPath]),
	})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return vaultClient, nil
}

func NewClientUsingEnv(ctx context.Context) (*vaultapi.Client, error) {
	for _, varName := range []string{vaultAddress, vaultToken, vaultCAPath} {
		if value, ok := os.LookupEnv(varName); !ok || value == "" {
			return nil, microerror.Maskf(executionFailedError, "%s environment variable must be set", varName)
		}
	}

	vaultClient, err := NewClient(Config{
		Address: os.Getenv(vaultAddress),
		Token:   os.Getenv(vaultToken),
		CAPath:  os.Getenv(vaultCAPath),
	})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return vaultClient, nil

}
