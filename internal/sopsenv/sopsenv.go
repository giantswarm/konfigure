package sopsenv

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/giantswarm/microerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/giantswarm/konfigure/internal/sopsenv/key"

	// GS stuff uses `kgs`-generated kubeconfigs that use
	// `oidc` auth provider. This import makes is possible to
	// run `konfigure` locally for troubleshooting purposes.
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	// Environment variables used to configure SOPS, and to point it to the
	// keys storage. This helps creating temporary and "isolated" environment
	// for SOPS, in a similar manner how Flux does it. This way we make sure
	// we do not interfere with Kustomize operations, by leaving key chains
	// under default locations expected by SOPS, nor by leaving env vars set
	// to some custom locations when Kustomize decrypts files.
	ageKeyFileVar = "SOPS_AGE_KEY_FILE"
	gnuPGHomeVar  = "GNUPGHOME"

	// `konfigure.giantswarm.io/data=sops-key` is used to fetch Kubernetes
	// Secrets with SOPS keys in order to import them to a temporary location.
	konfigureLabelKey   = "konfigure.giantswarm.io/data"
	konfigureLabelValue = "sops-keys"
	konfigureNamespace  = "giantswarm"
	konfigureTmpDirName = "konfigure-sops-"

	// Keys extensions supported
	secretPGPExt = ".asc"
	secretAgeExt = ".agekey"
)

type SOPSEnvConfig struct {
	K8sClient  kubernetes.Interface
	KeysDir    string
	KeysSource string
}

type SOPSEnv struct {
	cleanup    func()
	k8sClient  kubernetes.Interface
	keysDir    string
	keysSource string
}

// NewSOPSEnv creates SOPS environment configurator, it works according to the
// below combinations.
//
// 1. User expects to run SOPS against his local default keychains for GPG and AGE:
//      keysDir=""
//	    keysSource="local"
//
// 2. User expects to run SOPS against his custom keychains located under `path`:
//      keysDir="path"
//      keysSource="local"
//
// 3. User expects to run SOPS against  Kubernetes-downloaded keys stored at tmp location:
//      keysDir=""
//      keysSource="kubernetes"
//
// 4. User expects to run SOPS against Kubernetes-downloaded keys stored under `path`:
//      keysDir="path"
//      keysSource="kubernetes"
//
func NewSOPSEnv(config SOPSEnvConfig) (*SOPSEnv, error) {
	s := &SOPSEnv{
		keysDir:    config.KeysDir,
		keysSource: config.KeysSource,
	}

	if config.KeysSource == key.KeysSourceLocal {
		return s, nil
	}

	if config.KeysDir == "" {
		keysDir, err := os.MkdirTemp("", konfigureTmpDirName)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		s.keysDir = keysDir
		s.cleanup = func() { os.RemoveAll(keysDir) }
	}

	if config.K8sClient != nil {
		s.k8sClient = config.K8sClient
		return s, nil
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	s.k8sClient = k8sClient
	return s, nil
}

func (s *SOPSEnv) Cleanup() {
	if s.cleanup != nil {
		s.cleanup()
	}
}

func (s *SOPSEnv) GetKeysDir() string {
	return s.keysDir
}

// Setup sets up a self-contingent environment for PGP and AGE keys,
// and temporarily point SOPS to it, by exporting env vars
func (s *SOPSEnv) Setup(ctx context.Context) error {
	var err error

	// Empty keysDir means we running against user / system default
	// keychains, no need to point SOPS to a custom ones. In this mode
	// we also do not import keys from K8s
	if s.keysDir == "" {
		return nil
	}

	err = s.setEnv()
	if err != nil {
		return microerror.Mask(err)
	}

	// `local` keysSource means we running against local directory and
	// do not want to download keys from Kubernetes Secrets
	if s.k8sClient == nil {
		return nil
	}

	err = s.importKeys(ctx)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

// importKeys grabs Kubernetes Secrets matching selector and import PGP and AGE
// keys into desired location. The Secrets are expected to match the Flux secrets constraints,
// see https://fluxcd.io/docs/components/kustomize/kustomization/#decryption-secret-reference.
func (s *SOPSEnv) importKeys(ctx context.Context) error {
	var err error

	o := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", konfigureLabelKey, konfigureLabelValue),
	}

	secrets, err := s.k8sClient.CoreV1().Secrets(konfigureNamespace).List(ctx, o)
	if err != nil {
		return microerror.Mask(err)
	}

	// Let user know no Secrets have been found using selector. Better
	// to stop here than to go further and possibly fail on SOPS Decrypt.
	if len(secrets.Items) == 0 {
		return microerror.Maskf(
			secretNotFoundError,
			"no Kubernetes Secrets matching %s=%s selector found\n",
			konfigureLabelKey, konfigureLabelValue,
		)
	}

	ageKeysMap := map[string][]byte{}
	for _, secret := range secrets.Items {
		for k, v := range secret.Data {
			switch ext := filepath.Ext(k); ext {
			case secretPGPExt:
				args := []string{
					"--no-default-keyring",
					"--batch",
					"--import",
				}

				err, _, stderr := s.runGPGCmd(ctx, bytes.NewReader(v), args)
				if err != nil {
					return microerror.Maskf(pgpImportError, "failed to import key GnuPG keyring: \n %s", stderr.String())
				}
			case secretAgeExt:
				// Put keys into map to filter out duplicates and thus avoid
				// writting the same key multiple times into the keys.txt file
				ageKeysMap[string(v)] = v
			}
		}
	}

	err = s.writeKeysTxt(ctx, ageKeysMap)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

// RunGPGCmd runs GPG binary with given args and input.
// It is exporter mainly for re-using in tests
func (s *SOPSEnv) runGPGCmd(ctx context.Context, stdin io.Reader, args []string) (err error, stdout bytes.Buffer, stderr bytes.Buffer) {
	cmd := exec.Command("gpg", args...)
	cmd.Stdin = stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return
}

// setEnv exports GnuPGP and AGE environment variables telling
// where to find private keys
func (s *SOPSEnv) setEnv() error {
	var err error

	err = os.Setenv(gnuPGHomeVar, s.keysDir)
	if err != nil {
		return microerror.Mask(err)
	}
	err = os.Setenv(ageKeyFileVar, fmt.Sprintf("%s/%s", s.keysDir, "keys.txt"))
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

// writeKeysTxt writes AGE private key to the `keys.txt` file, see
// https://github.com/mozilla/sops#encrypting-using-age
func (s *SOPSEnv) writeKeysTxt(ctx context.Context, keys map[string][]byte) error {
	ageKeys := [][]byte{}
	{
		// Lets sort the keys
		keysStr := make([]string, 0, len(keys))
		for k := range keys {
			keysStr = append(keysStr, k)
		}
		sort.Strings(keysStr)

		for _, k := range keysStr {
			ageKeys = append(ageKeys, keys[k])
		}
	}

	keysTxt, err := os.OpenFile(fmt.Sprintf("%s/%s", s.keysDir, "keys.txt"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return microerror.Mask(err)
	}

	if _, err := keysTxt.Write(bytes.Join(ageKeys, []byte("\n"))); err != nil {
		return microerror.Mask(err)
	}

	if err := keysTxt.Close(); err != nil {
		return microerror.Mask(err)
	}

	return nil
}
