package sopsenv

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/giantswarm/microerror"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgofake "k8s.io/client-go/kubernetes/fake"

	"github.com/giantswarm/konfigure/internal/testutils"
)

func TestSetups(t *testing.T) {
	testCases := []struct {
		name          string
		config        SOPSEnvConfig
		expectCleanup bool
		expectedErr   error
		expectedVars  map[string]string
	}{
		{
			name: "default",
			config: SOPSEnvConfig{
				KeysSource: "local",
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
				KeysSource: "local",
			},
			expectedVars: map[string]string{
				gnuPGHomeVar:  tmpDirName("local"),
				ageKeyFileVar: tmpDirName("local") + "/keys.txt",
			},
		},
		{
			name: "kubernetes with dir given",
			config: SOPSEnvConfig{
				K8sClient: clientgofake.NewSimpleClientset(
					testutils.NewSecret("test", "giantswarm", true, map[string][]byte{}),
				),
				KeysDir:    tmpDirName("k8s"),
				KeysSource: "kubernetes",
			},
			expectedVars: map[string]string{
				gnuPGHomeVar:  tmpDirName("k8s"),
				ageKeyFileVar: tmpDirName("k8s") + "/keys.txt",
			},
		},
		{
			name: "kubernetes with dir generated",
			config: SOPSEnvConfig{
				K8sClient: clientgofake.NewSimpleClientset(
					testutils.NewSecret("test", "giantswarm", true, map[string][]byte{}),
				),
				KeysSource: "kubernetes",
			},
			expectCleanup: true,
		},
		{
			name: "kubernetes with no Secrets",
			config: SOPSEnvConfig{
				K8sClient:  clientgofake.NewSimpleClientset(),
				KeysSource: "kubernetes",
			},
			expectCleanup: true,
			expectedErr:   secretNotFoundError,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			var err error

			if tc.config.KeysDir != "" {
				err = os.Mkdir(tc.config.KeysDir, 0700)
				if err != nil {
					panic(err)
				}

				defer os.RemoveAll(tc.config.KeysDir)
			}

			se, cl, err := NewSOPSEnv(tc.config)
			if err != nil {
				t.Fatalf("error == %#v, want nil", err)
			}

			err = se.Setup(context.TODO())
			if err != nil && tc.expectedErr == nil {
				t.Fatalf("error == %#v, want nil", err)
			}

			if err != nil && tc.expectedErr != nil {
				if microerror.Cause(err) != tc.expectedErr {
					t.Fatalf("error == %#v, want %#v", err, tc.expectedErr)
				}
			}

			gotCleanup := cl != nil
			if gotCleanup != tc.expectCleanup {
				t.Fatalf("want cleanup: %t, got: %t", tc.expectCleanup, gotCleanup)
			}

			if gotCleanup {
				defer cl()
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
	var err error

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
		expectedKeysTxt []byte
		expectedPGPKeys []string
	}{
		{
			name: "flawless with tmp dir",
			secrets: []*corev1.Secret{
				testutils.NewSecret("regular-secret-1", "giantswarm", false, map[string][]byte{
					"password": []byte(`security`),
				}),
				testutils.NewSecret("regular-secret-2", "giantswarm", false, map[string][]byte{}),
				testutils.NewSecret("sops-gpg-keys-1", "giantswarm", true, map[string][]byte{
					"key1.agekey": testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
					"key1.asc":    testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
				testutils.NewSecret("sops-gpg-keys-2", "giantswarm", true, map[string][]byte{
					"key2.agekey": testutils.GetFile("testdata/keys/age1t60sj6dj77q7jp47s4tav4a967c8609lsexmg8eutxnez6d5gp8s27g9kl.private"),
				}),
			},
			expectedKeysTxt: testutils.GetFile("testdata/expected/keys1.txt"),
			expectedPGPKeys: []string{
				"F65B080F01DB7669363DFE31B69A68334353D9C0",
			},
		},
		{
			name: "duplicated keys",
			secrets: []*corev1.Secret{
				testutils.NewSecret("sops-gpg-keys-1", "giantswarm", true, map[string][]byte{
					"key1.agekey": testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
					"key1.asc":    testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
				testutils.NewSecret("sops-gpg-keys-2", "giantswarm", true, map[string][]byte{
					"key1.agekey": testutils.GetFile("testdata/keys/age1q3ed8z5e25t5a2vmzvzsyc9kevd68ukvuvajex0jwhewupat95zsdjmmrw.private"),
					"key1.asc":    testutils.GetFile("testdata/keys/F65B080F01DB7669363DFE31B69A68334353D9C0.private"),
				}),
			},
			expectedKeysTxt: testutils.GetFile("testdata/expected/keys2.txt"),
			expectedPGPKeys: []string{
				"F65B080F01DB7669363DFE31B69A68334353D9C0",
			},
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

			client := clientgofake.NewSimpleClientset(k8sObj...)

			var cl func()
			var se *SOPSEnv
			{
				seConfig := SOPSEnvConfig{
					K8sClient:  client,
					KeysDir:    "",
					KeysSource: "kubernetes",
				}

				se, cl, err = NewSOPSEnv(seConfig)
				if err != nil {
					t.Fatalf("error == %#v, want nil", err)
				}

				if cl != nil {
					defer cl()
				}
			}

			oldEnvs := map[string]string{
				ageKeyFileVar: os.Getenv(ageKeyFileVar),
				gnuPGHomeVar:  os.Getenv(gnuPGHomeVar),
			}

			err = se.Setup(context.TODO())
			if err != nil {
				t.Fatalf("error == %#v, want nil", err)
			}

			if os.Getenv(ageKeyFileVar) == oldEnvs[ageKeyFileVar] {
				t.Fatalf("wrong %s value, got %s=%s", ageKeyFileVar, ageKeyFileVar, oldEnvs[ageKeyFileVar])
			}

			if os.Getenv(gnuPGHomeVar) == oldEnvs[gnuPGHomeVar] {
				t.Fatalf("wrong %s value, got %s=%s", gnuPGHomeVar, gnuPGHomeVar, oldEnvs[gnuPGHomeVar])
			}

			keysTxt, err := ioutil.ReadFile(os.Getenv(ageKeyFileVar))
			if err != nil {
				t.Fatalf("error == %#v, want nil", err)
			}

			if !reflect.DeepEqual(keysTxt, tc.expectedKeysTxt) {
				t.Fatalf("want matching files \n %s", cmp.Diff(keysTxt, tc.expectedKeysTxt))
			}

			for _, fp := range tc.expectedPGPKeys {
				err, _, stderr := se.RunGPGCmd(
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
	path := os.TempDir() + konfigureTmpDirName + suffix

	return path
}
