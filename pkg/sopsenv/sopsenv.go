package sopsenv

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"filippo.io/age"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/giantswarm/konfigure/v2/pkg/sopsenv/key"

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

	// KonfigureLabelKey `konfigure.giantswarm.io/data=sops-key` is used to fetch Kubernetes
	// Secrets with SOPS keys in order to import them to a temporary location.
	KonfigureLabelKey   = "konfigure.giantswarm.io/data"
	KonfigureLabelValue = "sops-keys"
	konfigureTmpDirName = "konfigure-sops-"

	// Keys extensions supported
	secretPGPExt = ".asc"
	secretAgeExt = ".agekey"
)

// AllKeysNamespaces, given as the only entry of SOPSEnvConfig.KeysNamespaces,
// imports SOPS key Secrets from all namespaces.
const AllKeysNamespaces = "*"

// DefaultKeysNamespaces returns the namespaces SOPS key Secrets are imported
// from when SOPSEnvConfig.KeysNamespaces is empty.
func DefaultKeysNamespaces() []string {
	return []string{"giantswarm", "flux-giantswarm"}
}

type SOPSEnvConfig struct {
	K8sClient  kubernetes.Interface
	KeysDir    string
	KeysSource string
	Logger     logr.Logger

	// KeysNamespaces lists the namespaces to import labelled SOPS key
	// Secrets from when KeysSource is "kubernetes". Empty entries are
	// ignored, and DefaultKeysNamespaces is used when no entry is left.
	// AllKeysNamespaces as the only entry imports from all namespaces; only
	// use it when every namespace in the cluster is trusted to provide
	// decryption keys.
	KeysNamespaces []string
}

type SOPSEnv struct {
	cleanup        func()
	k8sClient      kubernetes.Interface
	keysDir        string
	keysNamespaces []string
	keysSource     string
	logger         logr.Logger
}

// NewSOPSEnv creates SOPS environment configurator, it works according to the
// below combinations.
//
//  1. User expects to run SOPS against his local default keychains for GPG and AGE:
//     keysDir=""
//     keysSource="local"
//
//  2. User expects to run SOPS against his custom keychains located under `path`:
//     keysDir="path"
//     keysSource="local"
//
//  3. User expects to run SOPS against  Kubernetes-downloaded keys stored at tmp location:
//     keysDir=""
//     keysSource="kubernetes"
//
//  4. User expects to run SOPS against Kubernetes-downloaded keys stored under `path`:
//     keysDir="path"
//     keysSource="kubernetes"
func NewSOPSEnv(config SOPSEnvConfig) (*SOPSEnv, error) {
	s := &SOPSEnv{
		keysDir:    config.KeysDir,
		keysSource: config.KeysSource,
		logger:     config.Logger,
	}

	if config.KeysSource == key.KeysSourceLocal {
		return s, nil
	}

	keysNamespaces, err := normalizeKeysNamespaces(config.KeysNamespaces)
	if err != nil {
		return nil, err
	}
	s.keysNamespaces = keysNamespaces

	if config.KeysDir == "" {
		keysDir, err := os.MkdirTemp("", konfigureTmpDirName)
		if err != nil {
			return nil, err
		}

		s.keysDir = keysDir
		s.cleanup = func() { _ = os.RemoveAll(keysDir) }
	}

	if config.K8sClient != nil {
		s.k8sClient = config.K8sClient
		return s, nil
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	s.k8sClient = k8sClient
	return s, nil
}

// normalizeKeysNamespaces drops empty and duplicate entries, applies the
// default and turns AllKeysNamespaces into metav1.NamespaceAll.
func normalizeKeysNamespaces(namespaces []string) ([]string, error) {
	result := make([]string, 0, len(namespaces))
	for _, namespace := range namespaces {
		namespace = strings.TrimSpace(namespace)
		if namespace != "" && !slices.Contains(result, namespace) {
			result = append(result, namespace)
		}
	}

	if len(result) == 0 {
		return DefaultKeysNamespaces(), nil
	}

	if slices.Contains(result, AllKeysNamespaces) {
		if len(result) != 1 {
			return nil, &InvalidConfigError{message: fmt.Sprintf("%q must be the only keys namespace when given", AllKeysNamespaces)}
		}
		return []string{metav1.NamespaceAll}, nil
	}

	for _, namespace := range result {
		if errs := validation.IsDNS1123Label(namespace); len(errs) != 0 {
			return nil, &InvalidConfigError{message: fmt.Sprintf("keys namespace %q is not a valid namespace name: %s", namespace, strings.Join(errs, ", "))}
		}
	}

	return result, nil
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

	// Empty keysDir means we are running against user / system default
	// keychains, no need to point SOPS to a custom ones. In this mode
	// we also do not import keys from K8s
	if s.keysDir == "" {
		return nil
	}

	err = s.setEnv()
	if err != nil {
		return err
	}

	// `local` keysSource means we are running against local directory and
	// do not want to download keys from Kubernetes Secrets
	if s.k8sClient == nil {
		return nil
	}

	err = s.importKeys(ctx)
	if err != nil {
		return err
	}

	return nil
}

// importKeys grabs Kubernetes Secrets matching selector in the configured
// namespaces and import PGP and AGE keys into desired location. The Secrets are
// expected to match the Flux secrets constraints,
// see https://fluxcd.io/docs/components/kustomize/kustomization/#decryption-secret-reference.
//
// A key that cannot be imported is logged and skipped, so one broken key does
// not prevent the others from being used. The import fails only when no key
// could be imported at all.
func (s *SOPSEnv) importKeys(ctx context.Context) error {
	if _, err := os.Stat(s.keysDir); os.IsNotExist(err) {
		return &NotFoundError{message: "specified keychains directory does not exist"}
	}

	o := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", KonfigureLabelKey, KonfigureLabelValue),
	}

	var secrets []corev1.Secret
	for _, namespace := range s.keysNamespaces {
		list, err := s.k8sClient.CoreV1().Secrets(namespace).List(ctx, o)
		if err != nil {
			return err
		}
		if len(list.Items) == 0 && namespace != metav1.NamespaceAll {
			s.logger.Info("warning: no SOPS key Secrets found in namespace, check the configured keys namespaces", "namespace", namespace, "selector", o.LabelSelector)
		}
		secrets = append(secrets, list.Items...)
	}

	// Let user know no Secrets have been found using selector.
	if len(secrets) == 0 {
		s.logger.Info(fmt.Sprintf("warning: no Kubernetes Secrets found matching selector: %s=%s, encrypted files cannot be decrypted", KonfigureLabelKey, KonfigureLabelValue), "namespaces", s.keysNamespaces)
		// Clear age keys written by an earlier import.
		return s.writeKeysTxt(ctx, nil)
	}

	imported := 0
	var failures []string

	ageKeysMap := map[string][]byte{}
	for _, secret := range secrets {
		for k, v := range secret.Data {
			source := fmt.Sprintf("%s/%s[%s]", secret.Namespace, secret.Name, k)

			switch ext := filepath.Ext(k); ext {
			case secretPGPExt:
				args := []string{
					"--no-default-keyring",
					"--batch",
					"--import",
				}

				_, stderr, err := s.runGPGCmd(ctx, bytes.NewReader(v), args)
				if err != nil {
					s.logger.Error(&PgpImportError{message: stderr.String()}, "skipping SOPS PGP key that failed to import", "source", source)
					failures = append(failures, source)
					continue
				}
				imported++
			case secretAgeExt:
				if err := validateAgeIdentities(v); err != nil {
					s.logger.Error(err, "skipping invalid SOPS age key", "source", source)
					failures = append(failures, source)
					continue
				}
				// Put keys into map to filter out duplicates and thus avoid
				// writing the same key multiple times into the keys.txt file
				ageKeysMap[string(v)] = v
				imported++
			}
		}
	}

	if imported == 0 && len(failures) > 0 {
		// Clear age keys written by an earlier import.
		if err := s.writeKeysTxt(ctx, nil); err != nil {
			return err
		}
		return &KeyImportError{message: fmt.Sprintf("failed to import any SOPS key, invalid keys in: %s", strings.Join(failures, ", "))}
	}

	return s.writeKeysTxt(ctx, ageKeysMap)
}

// validateAgeIdentities checks that every line of an age key file is empty, a
// comment or an identity SOPS can parse, so a broken key cannot make SOPS
// reject the whole keys.txt file.
func validateAgeIdentities(data []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	found := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "AGE-SECRET-KEY-1"):
			if _, err := age.ParseX25519Identity(line); err != nil {
				return &InvalidAgeKeyError{message: err.Error()}
			}
		default:
			// Only X25519 identities are supported. Plugin identities need
			// an age plugin binary, and SOPS stops at the first one it
			// cannot use, so they are refused as well.
			return &InvalidAgeKeyError{message: "unsupported identity type"}
		}

		found = true
	}

	if err := scanner.Err(); err != nil {
		return &InvalidAgeKeyError{message: err.Error()}
	}

	if !found {
		return &InvalidAgeKeyError{message: "no identity found"}
	}

	return nil
}

// RunGPGCmd runs GPG binary with given args and input.
// It is exporter mainly for re-using in tests
func (s *SOPSEnv) runGPGCmd(ctx context.Context, stdin io.Reader, args []string) (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	cmd := exec.Command("gpg", args...) //nolint:gosec // G204: fixed binary, args built from constants, key material goes via stdin
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
		return err
	}
	err = os.Setenv(ageKeyFileVar, fmt.Sprintf("%s/%s", s.keysDir, "keys.txt"))
	if err != nil {
		return err
	}

	return nil
}

// writeKeysTxt writes AGE private key to the `keys.txt` file, see
// https://github.com/mozilla/sops#encrypting-using-age
func (s *SOPSEnv) writeKeysTxt(ctx context.Context, keys map[string][]byte) error {
	ageKeys := [][]byte{}
	{
		// Let's sort the keys
		keysStr := make([]string, 0, len(keys))
		for k := range keys {
			keysStr = append(keysStr, k)
		}
		sort.Strings(keysStr)

		for _, k := range keysStr {
			ageKeys = append(ageKeys, keys[k])
		}
	}

	// Write to a temporary file and rename it over keys.txt, so readers
	// never see a partially written file.
	keysPath := filepath.Clean(filepath.Join(s.keysDir, "keys.txt"))
	tmp, err := os.CreateTemp(s.keysDir, "keys.txt.tmp-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}

	if _, err := tmp.Write(bytes.Join(ageKeys, []byte("\n"))); err != nil {
		_ = tmp.Close()
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, keysPath)
}
