package generator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giantswarm/konfigure/pkg/sopsenv"

	"github.com/giantswarm/konfigure/pkg/testutils"

	"github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
)

func TestGenerator_generateRawConfig(t *testing.T) {
	// This archive store development private keys. This is to avoid `gitleaks`
	// and `pre-commit` to complain on files stored in this repository. We untar
	// it here so that it can be used in test cases. Storing testing private keys
	// doesn't seem like a bad thing, since SOPS seems to do it as well, see:
	// AGE development key: https://raw.githubusercontent.com/mozilla/sops/master/age/keys.txt
	// PGP development key: https://raw.githubusercontent.com/mozilla/sops/master/pgp/sops_functional_tests_key.asc
	err := testutils.UntarFile("testdata/keys", "keys.tgz")
	if err != nil {
		t.Fatalf("error == %#v, want nil", err)
	}

	testCases := []struct {
		name                 string
		caseFile             string
		expectedErrorMessage string

		app          string
		installation string

		decryptTraverser DecryptTraverser

		secrets []*corev1.Secret
	}{
		{
			name:     "case 0 - basic config with config.yaml.patch",
			caseFile: "testdata/cases/case0.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 1 - include files in templates",
			caseFile: "testdata/cases/case1.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 2 - override global value for one installation",
			caseFile: "testdata/cases/case2.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 3 - keep non-string values after templating/patching",
			caseFile: "testdata/cases/case3.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 4 - allow templating in included files ",
			caseFile: "testdata/cases/case4.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 5 - test indentation when including files",
			caseFile: "testdata/cases/case5.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 6 - test app with no secrets (configmap only)",
			caseFile: "testdata/cases/case6.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 7 - patch configmap and secret",
			caseFile: "testdata/cases/case7.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 8 - decrypt secret data",
			caseFile: "testdata/cases/case8.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &mapStringTraverser{},
		},

		{
			name:                 "case 9 - throw error when a key is missing",
			caseFile:             "testdata/cases/case9.yaml",
			expectedErrorMessage: `<.this.key.is.missing>: map has no entry for key "this"`,

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &mapStringTraverser{},
		},

		{
			name:     "case 10 - no extra encoding for included files",
			caseFile: "testdata/cases/case10.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 11 - same as case 10 with SOPS GnuPGP encryption",
			caseFile: "testdata/cases/case11.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},

			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-keys", "giantswarm", true, map[string][]byte{
					"key.asc": testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
			},
		},

		{
			name:     "case 12 - same as case 10 with SOPS AGE encryption",
			caseFile: "testdata/cases/case12.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},

			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-keys", "giantswarm", true, map[string][]byte{
					"key.agekey": testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
				}),
			},
		},

		{
			name:     "case 13 - same as case 11, but with missing key",
			caseFile: "testdata/cases/case11.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},

			secrets: []*corev1.Secret{},

			expectedErrorMessage: `Error getting data key: 0 successful groups required, got 0`,
		},

		{
			name:     "case 14 - same as case 12, but with missing key",
			caseFile: "testdata/cases/case12.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},

			secrets: []*corev1.Secret{},

			expectedErrorMessage: `Error getting data key: 0 successful groups required, got 0`,
		},

		{
			name:     "case 15 - include self",
			caseFile: "testdata/cases/case15.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "konfigure-test")

			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			fs := testutils.NewMockFilesystem(tmpDir, tc.caseFile)

			// SOPS env setup from fake Kubernetes
			se, err := sopsenv.SetupNewSopsEnvironmentFromFakeKubernetes(tc.secrets)
			if err != nil {
				t.Fatalf("faled to setup SOPS environment: %s", err.Error())
			}

			defer se.Cleanup()

			config := Config{
				Fs:               fs,
				DecryptTraverser: tc.decryptTraverser,

				Installation: tc.installation,
			}
			g, err := New(config)
			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}
			configmap, secret, err := g.GenerateRawConfig(context.Background(), tc.app)
			if tc.expectedErrorMessage == "" {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			} else {
				switch {
				case err == nil:
					t.Fatalf("expected error %q but got nil", tc.expectedErrorMessage)
				case !strings.Contains(err.Error(), tc.expectedErrorMessage):
					t.Fatalf("expected error %q but got %q", tc.expectedErrorMessage, err)
				default:
					return
				}
			}
			if configmap != fs.ExpectedConfigmap {
				t.Fatalf("configmap not expected, got: %s", configmap)
			}
			if secret != fs.ExpectedSecret {
				t.Fatalf("secret not expected, got: %s", secret)
			}
		})
	}
}

func Test_sortYAMLKeys(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "konfigure-sort-yaml-keys-test")
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	fs := testutils.NewMockFilesystem(tmpDir, "testdata/cases/test_instances.yaml")

	config := Config{
		Fs:               fs,
		DecryptTraverser: &noopTraverser{},

		Installation: "puma",
	}
	g, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	// This compares first generated YAML to many subsequent generated YAML
	// to see if there is any difference in the keys order.
	var firstConfigMap string
	for i := 0; i < 100; i++ {
		configmap, _, err := g.GenerateRawConfig(context.Background(), "operator")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if firstConfigMap == "" {
			firstConfigMap = configmap
			continue
		}

		if configmap != firstConfigMap {
			f1 := filepath.Join(tmpDir, "cmp1")
			f2 := filepath.Join(tmpDir, "cmp2")
			for _, f := range []string{f1, f2} {
				if err := os.WriteFile(f, []byte(firstConfigMap), 0666); err != nil { // nolint:gosec
					t.Fatal("error creating file", f, err)
				}
			}
			cmd := exec.Command("git", "diff", "--exit-code", "--no-index", f1, f2) //nolint:gosec
			diff, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatal("error calling `git diff`", err)
			}
			t.Fatalf("configmap[%d] (diff): %s\n", i, diff)
		}
	}
}

type noopTraverser struct{}

func (t noopTraverser) Traverse(ctx context.Context, encrypted []byte) ([]byte, error) {
	return encrypted, nil
}

type mapStringTraverser struct{}

func (t mapStringTraverser) Traverse(ctx context.Context, encrypted []byte) ([]byte, error) {
	encryptedMap := map[string]string{}
	err := yaml.Unmarshal(encrypted, &encryptedMap)
	if err != nil {
		return []byte{}, err
	}

	decryptedMap := map[string]string{}
	for k, v := range encryptedMap {
		decryptedMap[k] = "decrypted-" + v
	}
	decrypted, err := yaml.Marshal(decryptedMap)
	if err != nil {
		return []byte{}, err
	}
	return decrypted, nil
}
