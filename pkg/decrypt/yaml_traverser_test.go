package decrypt

import (
	"context"
	"flag"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/giantswarm/microerror"
	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update .golden reference test files")

// Test_run uses golden files.
//
//	go test ./pkg/output -run TestYAMLTraverser -update
func TestYAMLTraverser(t *testing.T) {
	testCases := []struct {
		name               string
		inputFile          string
		expectedGoldenFile string
	}{
		{
			name:               "case 0: regular secret yaml",
			inputFile:          "secret.yaml.in",
			expectedGoldenFile: "secret.yaml.golden",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Log(tc.name)

			ctx := context.Background()

			var err error

			var input []byte
			{
				path := filepath.Join("testdata", tc.inputFile)
				input, err = ioutil.ReadFile(path)
				if err != nil {
					t.Fatalf("err = %#q, want %#v", microerror.Pretty(err, true), nil)
				}
			}

			var expectedResult []byte
			{
				path := filepath.Join("testdata", tc.expectedGoldenFile)
				expectedResult, err = ioutil.ReadFile(path)
				if err != nil {
					t.Fatalf("err = %#q, want %#v", microerror.Pretty(err, true), nil)
				}
			}

			var traverser *YAMLTraverser
			{
				c := YAMLTraverserConfig{
					Decrypter: &testDecrypter{},
				}

				traverser, err = NewYAMLTraverser(c)
				if err != nil {
					t.Fatalf("err = %#q, want %#v", microerror.Pretty(err, true), nil)
				}
			}

			result, err := traverser.Traverse(ctx, input)
			if err != nil {
				t.Fatalf("err = %#q, want %#v", microerror.Pretty(err, true), nil)
			}

			if *update {
				path := filepath.Join("testdata", tc.expectedGoldenFile)
				err := ioutil.WriteFile(path, []byte(result), 0644) // nolint:gosec
				if err != nil {
					t.Fatalf("err = %#q, want %#v", microerror.Pretty(err, true), nil)
				}
			}

			if !cmp.Equal(expectedResult, result) {
				t.Fatalf("\n\n%s\n", cmp.Diff(string(expectedResult), string(result)))
			}
		})
	}
}

type testDecrypter struct{}

func (d *testDecrypter) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	return []byte("decrypted"), nil
}
