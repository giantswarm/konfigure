package service

import (
	"os"
	"strings"
	"testing"

	"github.com/giantswarm/konfigure/v2/pkg/sopsenv"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"

	"github.com/giantswarm/konfigure/v2/pkg/testutils"
)

type RenderRawTestCase struct {
	name     string
	caseFile string

	schema string

	expectedErrorMessage string

	rawVariables []string

	secrets []*corev1.Secret
}

func TestRenderRaw_Legacy(t *testing.T) {
	err := testutils.UntarFile("testdata/keys", "keys.tgz")
	if err != nil {
		t.Fatalf("error == %#v, want nil", err)
	}

	testCases := []RenderRawTestCase{
		{
			name:     "case 0 - basic config with config.yaml.patch",
			caseFile: "testdata/legacy/cases/case0.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		{
			name:     "case 1 - include files in templates",
			caseFile: "testdata/legacy/cases/case1.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		{
			name:     "case 2 - override global value for one installation",
			caseFile: "testdata/legacy/cases/case2.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		{
			name:     "case 3 - keep non-string values after templating/patching",
			caseFile: "testdata/legacy/cases/case3.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		{
			name:     "case 4 - allow templating in included files ",
			caseFile: "testdata/legacy/cases/case4.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		{
			name:     "case 5 - test indentation when including files",
			caseFile: "testdata/legacy/cases/case5.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		{
			name:     "case 6 - test app with no secrets (configmap only)",
			caseFile: "testdata/legacy/cases/case6.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		{
			name:     "case 7 - patch configmap and secret",
			caseFile: "testdata/legacy/cases/case7.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		// case 8: the original case 8 does not make sense here cos it uses data from a mocked part of the generator
		{
			name:                 "case 9 - throw error when a key is missing",
			caseFile:             "testdata/legacy/cases/case9.yaml",
			expectedErrorMessage: `<.this.key.is.missing>: map has no entry for key "this"`,

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		{
			name:     "case 10 - no extra encoding for included files",
			caseFile: "testdata/legacy/cases/case10.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
		{
			name:     "case 11 - same as case 10 with SOPS GnuPGP encryption",
			caseFile: "testdata/legacy/cases/case11.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},

			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-keys", "giantswarm", true, map[string][]byte{
					"key.asc": testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
			},
		},
		{
			name:     "case 12 - same as case 10 with SOPS AGE encryption",
			caseFile: "testdata/legacy/cases/case12.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},

			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-keys", "giantswarm", true, map[string][]byte{
					"key.agekey": testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
				}),
			},
		},
		{
			name:     "case 13 - same as case 11, but with missing key",
			caseFile: "testdata/legacy/cases/case11.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},

			secrets: []*corev1.Secret{},

			expectedErrorMessage: `Error getting data key: 0 successful groups required, got 0`,
		},
		{
			name:     "case 14 - same as case 12, but with missing key",
			caseFile: "testdata/legacy/cases/case12.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},

			secrets: []*corev1.Secret{},

			expectedErrorMessage: `Error getting data key: 0 successful groups required, got 0`,
		},
		{
			name:     "case 15 - include self",
			caseFile: "testdata/legacy/cases/case15.yaml",

			schema: "testdata/legacy/schema.yaml",

			rawVariables: []string{"app=operator", "installation=puma"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RenderRawTestCore(t, tc)
		})
	}
}

func TestRenderRaw_Stages(t *testing.T) {
	err := testutils.UntarFile("testdata/keys", "keys.tgz")
	if err != nil {
		t.Fatalf("error == %#v, want nil", err)
	}

	testCases := []RenderRawTestCase{
		{
			name:     "case 0 - empty config",
			caseFile: "testdata/stages/cases/case0.yaml",

			schema: "testdata/stages/schema.yaml",

			rawVariables: []string{"stage=dev", "management-cluster=mc-1", "konfiguration=konfiguration-1"},
		},
		{
			name:     "case 1 - simple config",
			caseFile: "testdata/stages/cases/case1.yaml",

			schema: "testdata/stages/schema.yaml",

			rawVariables: []string{"stage=dev", "management-cluster=mc-1", "konfiguration=konfiguration-1"},
		},
		{
			name:     "case 2 - config with list overrides and patches for object and list",
			caseFile: "testdata/stages/cases/case2.yaml",

			schema: "testdata/stages/schema.yaml",

			rawVariables: []string{"stage=dev", "management-cluster=mc-1", "konfiguration=konfiguration-1"},
		},
		{
			name:     "case 3 - multiple stages and management-cluster overrides",
			caseFile: "testdata/stages/cases/case3.yaml",

			schema: "testdata/stages/schema.yaml",

			rawVariables: []string{"stage=production", "management-cluster=mc-2", "konfiguration=konfiguration-1"},
		},
		{
			name:     "case 4 - complex with SOPS and lots of patches",
			caseFile: "testdata/stages/cases/case4.yaml",

			schema: "testdata/stages/schema.yaml",

			rawVariables: []string{"stage=dev", "management-cluster=mc-1", "konfiguration=konfiguration-1"},

			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-keys", "giantswarm", true, map[string][]byte{
					"key.agekey": testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
				}),
			},
		},
		{
			name:     "case 5 - use custom include function",
			caseFile: "testdata/stages/cases/case5.yaml",

			schema: "testdata/stages/schema.yaml",

			rawVariables: []string{"stage=dev", "management-cluster=mc-1", "konfiguration=konfiguration-1"},
		},
		{
			name:     "case 6 - error on missing required template",
			caseFile: "testdata/stages/cases/case6.yaml",

			schema: "testdata/stages/schema.yaml",

			rawVariables: []string{"stage=dev", "management-cluster=mc-1", "konfiguration=konfiguration-1"},

			expectedErrorMessage: "0-base/konfiguration-1/config-map-template.yaml does not exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RenderRawTestCore(t, tc)
		})
	}
}

func RenderRawTestCore(t *testing.T, tc RenderRawTestCase) {
	tmpDir, err := os.MkdirTemp("", "konfigure-test")

	if err != nil {
		t.Fatalf("failed to create temp dir: %s", err.Error())
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// SOPS env setup from fake Kubernetes
	se, err := sopsenv.SetupNewSopsEnvironmentFromFakeKubernetes(tc.secrets)
	if err != nil {
		t.Fatalf("faled to create SOPS environment: %s", err.Error())
	}

	defer se.Cleanup()

	fs := testutils.NewMockFilesystem(tmpDir, tc.caseFile)

	service := NewDynamicService(DynamicServiceConfig{
		Log: logr.Discard(),
	})

	configmap, secret, err := service.RenderRaw(tmpDir, tc.schema, tc.rawVariables)

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
		t.Fatalf("configmap not expected, got: %s, expected: %s", configmap, fs.ExpectedConfigmap)
	}
	if secret != fs.ExpectedSecret {
		t.Fatalf("secret not expected, got: %s, expected: %s", secret, fs.ExpectedSecret)
	}
}
