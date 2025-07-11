package service

import (
	"os"
	"strings"
	"testing"

	"github.com/go-logr/logr"

	"github.com/giantswarm/konfigure/pkg/testutils"
)

func TestRenderRaw(t *testing.T) {
	err := testutils.UntarFile("../generator/testdata/keys", "keys.tgz")
	if err != nil {
		t.Fatalf("error == %#v, want nil", err)
	}

	testCases := []struct {
		name     string
		caseFile string

		expectedErrorMessage string

		app          string
		installation string

		ageKeyFile string
	}{
		{
			name:     "case 0 - basic config with config.yaml.patch",
			caseFile: "../generator/testdata/cases/case0.yaml",

			app:          "operator",
			installation: "puma",
		},
		{
			name:     "case 1 - include files in templates",
			caseFile: "../generator/testdata/cases/case1.yaml",

			app:          "operator",
			installation: "puma",
		},
		{
			name:     "case 2 - override global value for one installation",
			caseFile: "../generator/testdata/cases/case2.yaml",

			app:          "operator",
			installation: "puma",
		},
		{
			name:     "case 3 - keep non-string values after templating/patching",
			caseFile: "../generator/testdata/cases/case3.yaml",

			app:          "operator",
			installation: "puma",
		},
		{
			name:     "case 4 - allow templating in included files ",
			caseFile: "../generator/testdata/cases/case4.yaml",

			app:          "operator",
			installation: "puma",
		},
		{
			name:     "case 5 - test indentation when including files",
			caseFile: "../generator/testdata/cases/case5.yaml",

			app:          "operator",
			installation: "puma",
		},
		{
			name:     "case 6 - test app with no secrets (configmap only)",
			caseFile: "../generator/testdata/cases/case6.yaml",

			app:          "operator",
			installation: "puma",
		},
		{
			name:     "case 7 - patch configmap and secret",
			caseFile: "../generator/testdata/cases/case7.yaml",

			app:          "operator",
			installation: "puma",
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

			service := NewDynamicService(DynamicServiceConfig{
				Log: logr.Discard(),
			})

			configmap, secret, err := service.RenderRaw(tmpDir, "testdata/legacy/schema.yaml", []string{
				"app=" + tc.app,
				"installation=" + tc.installation,
			})

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
