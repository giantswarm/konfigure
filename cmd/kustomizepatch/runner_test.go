package kustomizepatch

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger/microloggertest"
)

func TestRunner_updateConfig(t *testing.T) {
	archive, err := os.ReadFile("testdata/latestchecksum.tar.gz")
	if err != nil {
		panic(err)
	}

	// Create HTTP servers for Source Controller and Kubernetes API
	srcCtrlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/gitrepository/flux-giantswarm/giantswarm-config/latestchecksum.tar.gz":
			w.WriteHeader(http.StatusOK)
			w.Write(archive)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer srcCtrlServer.Close()

	k8sServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/apis/source.toolkit.fluxcd.io/v1/namespaces/flux-giantswarm/gitrepositories/giantswarm-config":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":{"artifact":{"url":"` + srcCtrlServer.URL + "/gitrepository/flux-giantswarm/giantswarm-config/latestchecksum.tar.gz" + `"}}}`))
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer k8sServer.Close()

	// Parse servers' URLs for configuring the runner
	srcCtrlUrl, err := url.Parse(srcCtrlServer.URL)
	if err != nil {
		panic(err)
	}

	k8sUrl, err := url.Parse(k8sServer.URL)
	if err != nil {
		panic(err)
	}

	// Export appropriate environment variables to configure the runner
	t.Setenv("KONFIGURE_SOURCE_SERVICE", fmt.Sprintf("%s:%s", srcCtrlUrl.Hostname(), srcCtrlUrl.Port()))
	t.Setenv("KONFIGURE_GITREPO", "flux-giantswarm/giantswarm-config")
	t.Setenv("KUBERNETES_SERVICE_HOST", k8sUrl.Hostname())
	t.Setenv("KUBERNETES_SERVICE_PORT", k8sUrl.Port())

	testCases := []struct {
		name              string
		deprecatedPresent bool
		latestPresent     bool
	}{
		{
			name:              "fresh, no archive loaded",
			deprecatedPresent: false,
			latestPresent:     false,
		},
		{
			name:              "deprecated archive loaded, but newer available",
			deprecatedPresent: true,
			latestPresent:     false,
		},
		{
			name:              "deprecated archive loaded, but newer available",
			deprecatedPresent: true,
			latestPresent:     false,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			// Create a test cache directory
			tmpCacheDir, err := os.MkdirTemp("", "konfigure-test")
			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}
			defer os.RemoveAll(tmpCacheDir)

			if tc.latestPresent && !tc.deprecatedPresent {
				err = prePopulateCache(
					tmpCacheDir,
					[]byte(`latestchecksum.tar.gz`),
					[]byte(`newvalue`),
				)
				if err != nil {
					t.Fatalf("want nil, got error: %s", err.Error())
				}
			} else if !tc.latestPresent && tc.deprecatedPresent {
				err = prePopulateCache(
					tmpCacheDir,
					[]byte(`deprecatedchecksum.tar.gz`),
					[]byte(`oldvalue`),
				)
				if err != nil {
					t.Fatalf("want nil, got error: %s", err.Error())
				}
			} else if tc.latestPresent && tc.deprecatedPresent {
				t.Fatalf("bad input, only one archive can be loaded")
			}

			r := &runner{
				logger: microloggertest.New(),
				stdout: new(bytes.Buffer),
			}

			// run updateConfigWithParams
			err = r.updateConfigWithParams(tmpCacheDir, "testdata/token")
			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}

			configPath := path.Join(tmpCacheDir, "latest/config.yaml")
			_, err = os.Stat(configPath)
			if os.IsNotExist(err) {
				t.Fatalf("%s not found, expected to be created", configPath)
			}

			config, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}

			if strings.TrimSpace(string(config)) != "newvalue" {
				t.Fatalf("want '%s', got '%s'", "newvalue", string(config))
			}
		})
	}
}

func prePopulateCache(cache string, archive, config []byte) error {
	err := os.WriteFile(path.Join(cache, cacheLastArchive), archive, 0755) // nolint:gosec
	if err != nil {
		return microerror.Mask(err)
	}
	dir := path.Join(cache, "latest")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return microerror.Mask(err)
	}
	err = os.WriteFile(path.Join(dir, "config.yaml"), config, 0755) // nolint:gosec
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
