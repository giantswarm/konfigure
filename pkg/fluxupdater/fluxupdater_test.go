package fluxupdater

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
)

const advertisedTimestamp = "Thu, 02 Mar 2024 00:00:00 GMT"

func TestRunner_updateConfig(t *testing.T) {
	archive, err := os.ReadFile("testdata/latestrevision.tar.gz")
	if err != nil {
		panic(err)
	}

	// Create HTTP servers for Source Controller and Kubernetes API
	srcCtrlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/gitrepository/flux-giantswarm/giantswarm-config/latestrevision.tar.gz":
			w.Header().Add("Last-Modified", advertisedTimestamp)
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(archive)
			if err != nil {
				panic(err)
			}
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer srcCtrlServer.Close()

	k8sServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/apis/source.toolkit.fluxcd.io/v1/namespaces/flux-giantswarm/gitrepositories/giantswarm-config":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`{"status":{"artifact":{"url":"` + srcCtrlServer.URL + "/gitrepository/flux-giantswarm/giantswarm-config/latestrevision.tar.gz" + `"}}}`))
			if err != nil {
				panic(err)
			}
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

	testCases := []struct {
		name                    string
		deprecatedPresent       bool
		expectedConfigYamlValue string
		latestPresent           bool
		testArchiveName         []byte
		testArchiveTimestamp    []byte
		testArtifactUrl         []byte
		testConfigYaml          []byte
	}{
		{
			expectedConfigYamlValue: "newvalue",
			name:                    "fresh, no archive loaded",
		},
		{
			/*
				The archive is a composition of a customer part and shared part
				included by Flux. When customer part changes the revision advertised
				naturally changes as well, and hence the URL the Source Controller
				serves the archive at. This test covers this scenario. The timestamp
				is irrelevant in this case.
			*/
			expectedConfigYamlValue: "newvalue",
			name:                    "old revision archive loaded, newer available",
			testArchiveName:         []byte(`deprecatedrevision.tar.gz`),
			testArchiveTimestamp:    []byte(`Thu, 28 Feb 2024 00:00:00 GMT`),
			testArtifactUrl:         []byte(fmt.Sprintf("%s/gitrepository/flux-giantswarm/giantswarm-config/latestrevision.tar.gz", srcCtrlUrl)),
			testConfigYaml:          []byte(`oldvalue`),
		},
		{
			/*
				When the shared configs part changes, and the customer base does not,
				the revision stays the same. But the Last-Modified timestamp should
				change informing the archive has been updated. If so, it must be
				reloaded. This test covers this scenario.
			*/
			expectedConfigYamlValue: "newvalue",
			name:                    "latest revision loaded, modified since last time",
			testArchiveName:         []byte(`latestrevision.tar.gz`),
			testArchiveTimestamp:    []byte(`Thu, 01 Mar 2024 00:00:00 GMT`),
			testArtifactUrl:         []byte(fmt.Sprintf("%s/gitrepository/flux-giantswarm/giantswarm-config/latestrevision.tar.gz", srcCtrlUrl)),
			testConfigYaml:          []byte(`oldvalue`),
		},
		{
			/*
				Neither revision nor timestamp has changed
			*/
			expectedConfigYamlValue: "somevalue",
			name:                    "latest archive loaded",
			testArchiveName:         []byte(`latestrevision.tar.gz`),
			testArchiveTimestamp:    []byte(advertisedTimestamp),
			testArtifactUrl:         []byte(fmt.Sprintf("%s/gitrepository/flux-giantswarm/giantswarm-config/latestrevision.tar.gz", srcCtrlUrl)),
			testConfigYaml:          []byte(`somevalue`),
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			// Create a test cache directory
			tmpCacheDir, err := os.MkdirTemp("", "konfigure-test")
			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}
			defer func() { _ = os.RemoveAll(tmpCacheDir) }()

			err = prePopulateCache(
				tmpCacheDir,
				tc.testArchiveName,
				tc.testConfigYaml,
				tc.testArchiveTimestamp,
				tc.testArtifactUrl,
			)
			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}

			fluxUpdaterConfig := Config{
				CacheDir:            tmpCacheDir,
				ApiServerHost:       k8sUrl.Hostname(),
				ApiServerPort:       k8sUrl.Port(),
				KubernetesTokenFile: "testdata/token",
				GitRepository:       "flux-giantswarm/giantswarm-config",
			}

			fluxUpdater, err := New(fluxUpdaterConfig)
			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}

			// run updateConfigWithParams
			err = fluxUpdater.UpdateConfig()
			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}

			configPath := path.Join(tmpCacheDir, "latest/config.yaml")
			_, err = os.Stat(configPath)
			if os.IsNotExist(err) {
				t.Fatalf("%s not found, expected to be created", configPath)
			}

			config, err := os.ReadFile(path.Clean(configPath))
			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}

			if strings.TrimSpace(string(config)) != tc.expectedConfigYamlValue {
				t.Fatalf("want '%s', got '%s'", tc.expectedConfigYamlValue, string(config))
			}

			timestamp, err := os.ReadFile(path.Join(tmpCacheDir, cacheLastArchiveTimestamp)) //nolint:gosec
			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}

			if strings.TrimSpace(string(timestamp)) != advertisedTimestamp {
				t.Fatalf("want '%s', got '%s'", advertisedTimestamp, string(timestamp))
			}
		})
	}
}

func prePopulateCache(cache string, archive, config, timestamp, url []byte) error {
	var err error
	if len(timestamp) > 0 {
		err = os.WriteFile(path.Join(cache, cacheLastArchive), archive, 0755) // nolint:gosec
		if err != nil {
			return err
		}
	}

	dir := path.Join(cache, "latest")
	if err := os.MkdirAll(dir, 0755); err != nil { //nolint:gosec
		return err
	}

	if len(timestamp) > 0 {
		err = os.WriteFile(path.Join(dir, "config.yaml"), config, 0755) // nolint:gosec
		if err != nil {
			return err
		}
	}

	if len(timestamp) > 0 {
		err = os.WriteFile(path.Join(cache, cacheLastArchiveTimestamp), timestamp, 0755) // nolint:gosec
		if err != nil {
			return err
		}
	}

	if len(timestamp) > 0 {
		err = os.WriteFile(path.Join(cache, cacheLastArtifactUrl), url, 0755) // nolint:gosec
		if err != nil {
			return err
		}
	}

	return nil
}
