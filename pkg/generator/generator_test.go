package generator

import (
	"context"
	_ "embed"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/giantswarm/microerror"
)

func TestGenerator_generateRawConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                 string
		caseFile             string
		expectedErrorMessage string

		app          string
		installation string

		decryptTraverser DecryptTraverser
	}{
		{
			name:     "case 0 - basic config with config.yaml.patch",
			caseFile: "testdata/case0.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 1 - include files in templates",
			caseFile: "testdata/case1.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 2 - override global value for one installation",
			caseFile: "testdata/case2.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 3 - keep non-string values after templating/patching",
			caseFile: "testdata/case3.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 4 - allow templating in included files ",
			caseFile: "testdata/case4.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 5 - test indentation when including files",
			caseFile: "testdata/case5.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 6 - test app with no secrets (configmap only)",
			caseFile: "testdata/case6.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 7 - patch configmap and secret",
			caseFile: "testdata/case7.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},

		{
			name:     "case 8 - decrypt secret data",
			caseFile: "testdata/case8.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &mapStringTraverser{},
		},

		{
			name:                 "case 9 - throw error when a key is missing",
			caseFile:             "testdata/case9.yaml",
			expectedErrorMessage: `<.this.key.is.missing>: map has no entry for key "this"`,

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &mapStringTraverser{},
		},

		{
			name:     "case 10 - no extra encoding for included files",
			caseFile: "testdata/case10.yaml",

			app:              "operator",
			installation:     "puma",
			decryptTraverser: &noopTraverser{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "konfigure-test")
			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}
			defer os.RemoveAll(tmpDir)

			fs := newMockFilesystem(tmpDir, tc.caseFile)

			config := Config{
				Fs:               fs,
				DecryptTraverser: tc.decryptTraverser,

				Installation: tc.installation,
			}
			g, err := New(config)
			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}

			configmap, secret, err := g.generateRawConfig(context.Background(), tc.app)
			if tc.expectedErrorMessage == "" {
				if err != nil {
					t.Fatalf("unexpected error: %s", microerror.Pretty(err, true))
				}
			} else {
				switch {
				case err == nil:
					t.Fatalf("expected error %q but got nil", tc.expectedErrorMessage)
				case !strings.Contains(microerror.Pretty(err, true), tc.expectedErrorMessage):
					t.Fatalf("expected error %q but got %q", tc.expectedErrorMessage, microerror.Pretty(err, true))
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

//go:embed testdata/test_instances.yaml
var sortYAMLKeysTestInstancesYAML string

func Test_sortYAMLKeys(t *testing.T) {
	t.Parallel()

	tmpDir, err := ioutil.TempDir("", "konfigure-sort-yaml-keys-test")
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	defer os.RemoveAll(tmpDir)

	fs := newMockFilesystem(tmpDir, "testdata/test_instances.yaml")

	config := Config{
		Fs:               fs,
		DecryptTraverser: &noopTraverser{},

		Installation: "puma",
	}
	g, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	var firstConfigMap string
	for i := 0; i < 100; i++ {
		configmap, _, err := g.generateRawConfig(context.Background(), "operator")
		if err != nil {
			t.Fatalf("unexpected error: %s", microerror.Pretty(err, true))
		}

		if firstConfigMap == "" {
			firstConfigMap = configmap
			continue
		}

		if configmap != firstConfigMap {
			f1 := filepath.Join(tmpDir, "cmp1")
			f2 := filepath.Join(tmpDir, "cmp2")
			os.WriteFile(f1, []byte(firstConfigMap), 0666)
			os.WriteFile(f2, []byte(configmap), 0666)
			t.Logf("configmap[%d] (diff) = \n", i)
			cmd := exec.Command("git", "diff", "--exit-code", "--no-index", f1, f2)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err != nil {
				t.Fatalf("err = %#q, want %#v", microerror.Pretty(err, true), nil)
			}
			t.Fatal()
		}
	}
}

func Test_sortYAMLKeys_null(t *testing.T) {
	t.Parallel()

	out, err := sortYAMLKeys("")
	if err != nil {
		t.Fatalf("err = %#q, want %#v", microerror.Pretty(err, true), nil)
	}

	if out != "" {
		t.Fatalf("out = %v, want %v", out, "")
	}
}

type mockFilesystem struct {
	tempDirPath string

	ExpectedConfigmap string
	ExpectedSecret    string
}

type testFile struct {
	Path string `json:"path"`
	Data string `json:"data"`
}

func newMockFilesystem(temporaryDirectory, caseFile string) *mockFilesystem {
	fs := mockFilesystem{
		tempDirPath: temporaryDirectory,
	}
	for _, p := range []string{"default", "installations", "include"} {
		if err := os.MkdirAll(path.Join(temporaryDirectory, p), 0755); err != nil {
			panic(err)
		}
	}

	rawData, err := ioutil.ReadFile(caseFile)
	if err != nil {
		panic(err)
	}

	splitFiles := strings.Split(string(rawData), "---")

	for _, rawYaml := range splitFiles {
		file := testFile{}
		if err := yaml.Unmarshal([]byte(rawYaml), &file); err != nil {
			panic(err)
		}

		p := path.Join(temporaryDirectory, file.Path)
		dir, filename := path.Split(p)

		switch filename {
		case "configmap-values.yaml.golden":
			fs.ExpectedConfigmap = file.Data
			continue
		case "secret-values.yaml.golden":
			fs.ExpectedSecret = file.Data
			continue
		}

		if err := os.MkdirAll(dir, 0755); err != nil {
			panic(err)
		}

		err := ioutil.WriteFile(p, []byte(file.Data), 0644) // nolint:gosec
		if err != nil {
			panic(err)
		}
	}

	return &fs
}

func (fs *mockFilesystem) ReadFile(filepath string) ([]byte, error) {
	data, err := ioutil.ReadFile(path.Join(fs.tempDirPath, filepath))
	if err != nil {
		return []byte{}, microerror.Maskf(notFoundError, "%q not found", filepath)
	}
	return data, nil
}

func (fs *mockFilesystem) ReadDir(dirpath string) ([]os.FileInfo, error) {
	p := path.Join(fs.tempDirPath, dirpath)
	return ioutil.ReadDir(p)
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
		return []byte{}, microerror.Mask(err)
	}

	decryptedMap := map[string]string{}
	for k, v := range encryptedMap {
		decryptedMap[k] = "decrypted-" + v
	}
	decrypted, err := yaml.Marshal(decryptedMap)
	if err != nil {
		return []byte{}, microerror.Mask(err)
	}
	return decrypted, nil
}
