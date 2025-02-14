package decrypt

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

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
				input, err = os.ReadFile(path)
				if err != nil {
					t.Fatalf("err = %#q, want %#v", err, nil)
				}
			}

			var expectedResult []byte
			{
				path := filepath.Join("testdata", tc.expectedGoldenFile)
				expectedResult, err = os.ReadFile(path)
				if err != nil {
					t.Fatalf("err = %#q, want %#v", err, nil)
				}
			}

			var traverser *YAMLTraverser
			{
				c := YAMLTraverserConfig{
					Decrypter: &testDecrypter{},
				}

				traverser, err = NewYAMLTraverser(c)
				if err != nil {
					t.Fatalf("err = %#q, want %#v", err, nil)
				}
			}

			result, err := traverser.Traverse(ctx, input)
			if err != nil {
				t.Fatalf("err = %#q, want %#v", err, nil)
			}

			if *update {
				path := filepath.Join("testdata", tc.expectedGoldenFile)
				err := os.WriteFile(path, []byte(result), 0644) // nolint:gosec
				if err != nil {
					t.Fatalf("err = %#q, want %#v", err, nil)
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
