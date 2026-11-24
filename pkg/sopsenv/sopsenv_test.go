package sopsenv

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgofake "k8s.io/client-go/kubernetes/fake"

	"github.com/giantswarm/konfigure/v2/pkg/sopsenv/key"
	"github.com/giantswarm/konfigure/v2/pkg/testutils"
)

const (
	ageKeyName     = "key1.agekey"
	pgpKeyName     = "key1.asc"
	pgpFingerprint = "F65B080F01DB7669363DFE31B69A68334353D9C0"

	ageKey2Name = "key2.agekey"
	gsNamespace = "giantswarm"

	// Split so that private key detectors do not flag these invalid keys.
	brokenAgeKey = "AGE-SECRET-KEY-" + "1NOTAKEY\n"
	pluginAgeKey = "AGE-PLUGIN-" + "YUBIKEY-1QQQQQQ\n"
	brokenPGPKey = "-----BEGIN PGP " + "PRIVATE KEY BLOCK-----\ngarbage\n-----END PGP " + "PRIVATE KEY BLOCK-----\n"

	ageKey1Path = "testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"
	ageKey2Path = "testdata/keys/age1t60sj6dj77q7jp47s4tav4a967c8609lsexmg8eutxnez6d5gp8s27g9kl.private"
	pgpKeyPath  = "testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"
)

func TestSetups(t *testing.T) {
	logger := logr.Discard()

	testCases := []struct {
		name          string
		config        SOPSEnvConfig
		dontCreateDir bool
		expectCleanup bool
		expectedVars  map[string]string
		expectedError error
	}{
		{
			name: "default",
			config: SOPSEnvConfig{
				KeysSource: key.KeysSourceLocal,
				Logger:     logger,
			},
			expectedVars: map[string]string{
				gnuPGHomeVar:  "",
				ageKeyFileVar: "",
			},
		},
		{
			name: "local with dir given",
			config: SOPSEnvConfig{
				KeysDir:    tmpDirName("local"),
				KeysSource: key.KeysSourceLocal,
				Logger:     logger,
			},
			expectedVars: map[string]string{
				gnuPGHomeVar:  tmpDirName("local"),
				ageKeyFileVar: tmpDirName("local") + "/keys.txt",
			},
		},
		{
			name: "kubernetes with dir given",
			config: SOPSEnvConfig{
				K8sClient: clientgofake.NewClientset(
					testutils.NewSecret("test", gsNamespace, true, map[string][]byte{}),
				),
				KeysDir:    tmpDirName("k8s"),
				KeysSource: key.KeysSourceKubernetes,
				Logger:     logger,
			},
			expectedVars: map[string]string{
				gnuPGHomeVar:  tmpDirName("k8s"),
				ageKeyFileVar: tmpDirName("k8s") + "/keys.txt",
			},
		},
		{
			name: "kubernetes with dir generated",
			config: SOPSEnvConfig{
				K8sClient: clientgofake.NewClientset(
					testutils.NewSecret("test", gsNamespace, true, map[string][]byte{}),
				),
				KeysSource: key.KeysSourceKubernetes,
				Logger:     logger,
			},
			expectCleanup: true,
		},
		{
			name: "kubernetes with no Secrets",
			config: SOPSEnvConfig{
				K8sClient:  clientgofake.NewClientset(),
				KeysSource: key.KeysSourceKubernetes,
				Logger:     logger,
			},
			expectCleanup: true,
		},
		{
			name: "local with non existing dir",
			config: SOPSEnvConfig{
				K8sClient:  clientgofake.NewClientset(),
				KeysDir:    "/non/existing/directory",
				KeysSource: key.KeysSourceKubernetes,
				Logger:     logger,
			},
			dontCreateDir: true,
			expectedError: &NotFoundError{},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			var err error

			if !tc.dontCreateDir && tc.config.KeysDir != "" {
				err = os.Mkdir(tc.config.KeysDir, 0700)
				if err != nil {
					panic(err)
				}

				defer func() { _ = os.RemoveAll(tc.config.KeysDir) }()
			}

			se, err := NewSOPSEnv(tc.config)
			if err != nil {
				t.Fatalf("error == %#v, want nil", err)
			}

			err = se.Setup(context.TODO())
			if tc.expectedError != nil {
				if !errors.Is(err, tc.expectedError) {
					t.Fatalf("error not matching expected matcher, got: %s", err)
				}
			} else if err != nil {
				t.Fatalf("error == %#v, want nil", err)
			}
			defer se.Cleanup()

			gotCleanup := se.cleanup != nil
			if gotCleanup != tc.expectCleanup {
				t.Fatalf("want cleanup: %t, got: %t", tc.expectCleanup, gotCleanup)
			}

			for k, v := range tc.expectedVars {
				if os.Getenv(k) != v {
					t.Fatalf("want %s=%s, got %s=%s", k, v, k, os.Getenv(k))
				}
			}
		})
	}
}

func TestImportKeys(t *testing.T) {
	logger := logr.Discard()

	var err error

	// This archive store development private keys. This is to avoid `gitleaks`
	// and `pre-commit` to complain on files stored in this repository. We untar
	// it here so that it can be used in test cases. Storing testing private keys
	// doesn't seem like a bad thing, since SOPS seems to do it as well, see:
	// AGE development key: https://raw.githubusercontent.com/mozilla/sops/master/age/keys.txt
	// PGP development key: https://raw.githubusercontent.com/mozilla/sops/master/pgp/sops_functional_tests_key.asc
	err = testutils.UntarFile("testdata/keys", "keys.tgz")
	if err != nil {
		t.Fatalf("error == %#v, want nil", err)
	}

	err = testutils.UntarFile("testdata/expected", "expected.tgz")
	if err != nil {
		t.Fatalf("error == %#v, want nil", err)
	}

	testCases := []struct {
		name            string
		secrets         []*corev1.Secret
		keysNamespaces  []string
		expectedKeysTxt []byte
		expectedPGPKeys []string
		expectedError   error
	}{
		{
			name: "flawless with tmp dir",
			secrets: []*corev1.Secret{
				testutils.NewSecret("regular-secret-1", gsNamespace, false, map[string][]byte{
					"password": []byte(`security`),
				}),
				testutils.NewSecret("regular-secret-2", gsNamespace, false, map[string][]byte{}),
				testutils.NewSecret("sops-gpg-keys-1", gsNamespace, true, map[string][]byte{
					ageKeyName: testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
					pgpKeyName: testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
				testutils.NewSecret("sops-gpg-keys-2", gsNamespace, true, map[string][]byte{
					ageKey2Name: testutils.GetFile("testdata/keys/age1t60sj6dj77q7jp47s4tav4a967c8609lsexmg8eutxnez6d5gp8s27g9kl.private"),
				}),
			},
			expectedKeysTxt: testutils.GetFile("testdata/expected/keys1.txt"),
			expectedPGPKeys: []string{
				pgpFingerprint,
			},
		},
		{
			name: "duplicated keys",
			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-gpg-keys-1", gsNamespace, true, map[string][]byte{
					ageKeyName: testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
					pgpKeyName: testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
				testutils.NewSecret("sops-gpg-keys-2", gsNamespace, true, map[string][]byte{
					ageKeyName: testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
					pgpKeyName: testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
			},
			expectedKeysTxt: testutils.GetFile("testdata/expected/keys2.txt"),
			expectedPGPKeys: []string{
				pgpFingerprint,
			},
		},
		{
			name: "duplicated keys",
			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-gpg-keys-1", gsNamespace, true, map[string][]byte{
					ageKeyName: testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
					pgpKeyName: testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
				testutils.NewSecret("sops-gpg-keys-2", "flux-giantswarm", true, map[string][]byte{
					ageKeyName: testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
					pgpKeyName: testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
			},
			expectedKeysTxt: testutils.GetFile("testdata/expected/keys2.txt"),
			expectedPGPKeys: []string{
				pgpFingerprint,
			},
		},
		{
			name: "key Secret in a foreign namespace is ignored",
			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-gpg-keys-1", gsNamespace, true, map[string][]byte{
					ageKeyName: testutils.GetFile(ageKey1Path),
					pgpKeyName: testutils.GetFile(pgpKeyPath),
				}),
				testutils.NewSecret("sops-gpg-keys-2", "org-x", true, map[string][]byte{
					ageKey2Name: testutils.GetFile(ageKey2Path),
				}),
			},
			expectedKeysTxt: testutils.GetFile("testdata/expected/keys2.txt"),
			expectedPGPKeys: []string{
				pgpFingerprint,
			},
		},
		{
			name: "key Secrets in all namespaces when asked for explicitly",
			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-gpg-keys-1", gsNamespace, true, map[string][]byte{
					ageKeyName: testutils.GetFile(ageKey1Path),
					pgpKeyName: testutils.GetFile(pgpKeyPath),
				}),
				testutils.NewSecret("sops-gpg-keys-2", "org-x", true, map[string][]byte{
					ageKey2Name: testutils.GetFile(ageKey2Path),
				}),
			},
			keysNamespaces:  []string{AllKeysNamespaces},
			expectedKeysTxt: testutils.GetFile("testdata/expected/keys1.txt"),
			expectedPGPKeys: []string{
				pgpFingerprint,
			},
		},
		{
			name: "broken keys are skipped",
			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-gpg-keys-1", gsNamespace, true, map[string][]byte{
					ageKeyName: testutils.GetFile(ageKey1Path),
					pgpKeyName: testutils.GetFile(pgpKeyPath),
				}),
				testutils.NewSecret("sops-gpg-keys-2", "flux-giantswarm", true, map[string][]byte{
					"broken.agekey":  []byte(brokenAgeKey),
					"plugin.agekey":  []byte(pluginAgeKey),
					"garbage.agekey": []byte("not a key at all\n"),
					"empty.agekey":   []byte(""),
					"broken.asc":     []byte(brokenPGPKey),
					"unrelated.txt":  []byte("ignored"),
				}),
			},
			expectedKeysTxt: testutils.GetFile("testdata/expected/keys2.txt"),
			expectedPGPKeys: []string{
				pgpFingerprint,
			},
		},
		{
			name: "only broken keys",
			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-gpg-keys-1", gsNamespace, true, map[string][]byte{
					"broken.agekey": []byte(brokenAgeKey),
				}),
			},
			expectedError: &KeyImportError{},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			k8sObj := make([]runtime.Object, 0)
			{
				for _, sec := range tc.secrets {
					k8sObj = append(k8sObj, sec)
				}
			}

			client := clientgofake.NewClientset(k8sObj...)

			var se *SOPSEnv
			{
				seConfig := SOPSEnvConfig{
					K8sClient:      client,
					KeysDir:        "",
					KeysSource:     key.KeysSourceKubernetes,
					KeysNamespaces: tc.keysNamespaces,
					Logger:         logger,
				}

				se, err = NewSOPSEnv(seConfig)
				if err != nil {
					t.Fatalf("error == %#v, want nil", err)
				}
				defer se.Cleanup()
			}

			oldEnvs := map[string]string{
				ageKeyFileVar: os.Getenv(ageKeyFileVar),
				gnuPGHomeVar:  os.Getenv(gnuPGHomeVar),
			}

			err = se.Setup(context.TODO())
			if tc.expectedError != nil {
				if !errors.Is(err, tc.expectedError) {
					t.Fatalf("want error matching %T, got %v", tc.expectedError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error == %#v, want nil", err)
			}

			if os.Getenv(ageKeyFileVar) == oldEnvs[ageKeyFileVar] {
				t.Fatalf("wrong %s value, got %s=%s", ageKeyFileVar, ageKeyFileVar, oldEnvs[ageKeyFileVar])
			}

			if os.Getenv(gnuPGHomeVar) == oldEnvs[gnuPGHomeVar] {
				t.Fatalf("wrong %s value, got %s=%s", gnuPGHomeVar, gnuPGHomeVar, oldEnvs[gnuPGHomeVar])
			}

			keysTxt, err := os.ReadFile(os.Getenv(ageKeyFileVar)) //nolint:gosec // G703: Setup points this at the test's own temp dir
			if err != nil {
				t.Fatalf("error == %#v, want nil", err)
			}

			if !reflect.DeepEqual(keysTxt, tc.expectedKeysTxt) {
				t.Fatalf("want matching files \n %s", cmp.Diff(keysTxt, tc.expectedKeysTxt))
			}

			for _, fp := range tc.expectedPGPKeys {
				_, stderr, err := se.runGPGCmd(
					context.TODO(),
					bytes.NewReader([]byte{}),
					[]string{"--list-secret-key", fp},
				)

				if err != nil {
					t.Fatalf("want %s key in keyring, got \n %s", fp, stderr.String())
				}
			}
		})
	}
}

func tmpDirName(suffix string) string {
	path := filepath.Join(os.TempDir(), konfigureTmpDirName+suffix)
	return path
}

func TestNormalizeKeysNamespaces(t *testing.T) {
	testCases := []struct {
		name      string
		input     []string
		expected  []string
		expectErr bool
	}{
		{name: "nil", input: nil, expected: DefaultKeysNamespaces()},
		{name: "only empty entries", input: []string{"", " "}, expected: DefaultKeysNamespaces()},
		{name: "empty entries dropped", input: []string{gsNamespace, ""}, expected: []string{gsNamespace}},
		{name: "duplicates dropped", input: []string{gsNamespace, gsNamespace, "org-x"}, expected: []string{gsNamespace, "org-x"}},
		{name: "all namespaces", input: []string{AllKeysNamespaces}, expected: []string{""}},
		{name: "all namespaces mixed with others", input: []string{AllKeysNamespaces, gsNamespace}, expectErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeKeysNamespaces(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("want %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestImportKeys_ClearsStaleKeysTxt(t *testing.T) {
	testCases := []struct {
		name    string
		secrets []*corev1.Secret
		wantErr bool
	}{
		{name: "no key Secrets left"},
		{
			name: "only broken keys left",
			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-keys", gsNamespace, true, map[string][]byte{
					ageKeyName: []byte(brokenAgeKey),
				}),
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keysDir := t.TempDir()
			keysTxtPath := filepath.Join(keysDir, "keys.txt")
			if err := os.WriteFile(keysTxtPath, []byte("stale key\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			k8sObj := make([]runtime.Object, 0, len(tc.secrets))
			for _, sec := range tc.secrets {
				k8sObj = append(k8sObj, sec)
			}

			se, err := NewSOPSEnv(SOPSEnvConfig{
				K8sClient:  clientgofake.NewClientset(k8sObj...),
				KeysDir:    keysDir,
				KeysSource: key.KeysSourceKubernetes,
				Logger:     logr.Discard(),
			})
			if err != nil {
				t.Fatal(err)
			}

			err = se.Setup(context.TODO())
			if tc.wantErr != (err != nil) {
				t.Fatalf("want error: %t, got %v", tc.wantErr, err)
			}

			keysTxt, err := os.ReadFile(keysTxtPath) //nolint:gosec // test temp dir
			if err != nil {
				t.Fatal(err)
			}
			if len(keysTxt) != 0 {
				t.Fatalf("want empty keys.txt, got %q", string(keysTxt))
			}
		})
	}
}

func TestNewSOPSEnv_KeysNamespacesCheckedForKubernetesOnly(t *testing.T) {
	invalid := []string{AllKeysNamespaces, gsNamespace}

	if _, err := NewSOPSEnv(SOPSEnvConfig{KeysSource: key.KeysSourceLocal, KeysNamespaces: invalid, Logger: logr.Discard()}); err != nil {
		t.Fatalf("want nil for local keys source, got %v", err)
	}

	se, err := NewSOPSEnv(SOPSEnvConfig{K8sClient: clientgofake.NewClientset(), KeysSource: key.KeysSourceKubernetes, KeysNamespaces: invalid, Logger: logr.Discard()})
	if err == nil {
		se.Cleanup()
		t.Fatal("want error for kubernetes keys source, got nil")
	}
}
