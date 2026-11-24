package fluxupdater

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	advertisedTimestamp = "Thu, 02 Mar 2024 00:00:00 GMT"
	newConfigValue      = "newvalue"
	testGitRepository   = "flux-giantswarm/giantswarm-config"
	testApiServerHost   = "kubernetes.default.svc"
	testTokenFile       = "testdata/token"

	testGitRepositoryAPIPath = "/apis/source.toolkit.fluxcd.io/v1/namespaces/flux-giantswarm/gitrepositories/giantswarm-config"
)

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
		case testGitRepositoryAPIPath:
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
			expectedConfigYamlValue: newConfigValue,
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
			expectedConfigYamlValue: newConfigValue,
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
			expectedConfigYamlValue: newConfigValue,
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
				KubernetesTokenFile: testTokenFile,
				GitRepository:       testGitRepository,
				ArtifactHosts:       []string{srcCtrlUrl.Hostname()},
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

func TestValidateArtifactURL(t *testing.T) {
	updater, err := New(Config{
		CacheDir:      t.TempDir(),
		ApiServerHost: testApiServerHost,
		ApiServerPort: "443",
		GitRepository: testGitRepository,
	})
	if err != nil {
		t.Fatalf("want nil, got error: %s", err.Error())
	}

	testCases := []struct {
		name  string
		url   string
		valid bool
		want  string
	}{
		{"default host", "http://source-controller.flux-giantswarm.svc.cluster.local/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true, ""},
		{"default host with trailing dot", "http://source-controller.flux-giantswarm.svc.cluster.local./gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true, ""},
		{"default host with port", "http://source-controller.flux-giantswarm.svc.cluster.local:80/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true, ""},
		{"https", "https://source-controller.flux-giantswarm.svc.cluster.local/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true, ""},
		{"short service name is completed", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true,
			"http://source-controller.flux-giantswarm.svc.cluster.local/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz"},
		{"short service name with trailing dot and port is completed", "http://Source-Controller.flux-giantswarm.svc.:9090/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true,
			"http://source-controller.flux-giantswarm.svc.cluster.local:9090/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz"},
		{"short service name of another service", "http://other.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false, ""},
		{"other cluster domain", "http://source-controller.flux-giantswarm.svc.example.internal/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false, ""},
		{"foreign host", "http://other.example.com/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false, ""},
		{"source controller in another namespace", "http://source-controller.flux-system.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false, ""},
		{"ip address", "http://10.0.0.1/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false, ""},
		{"user info", "http://source-controller.flux-giantswarm.svc@other.example.com/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false, ""},
		{"other repository", "http://source-controller.flux-giantswarm.svc/gitrepository/org-x/repo/abc.tar.gz", false, ""},
		{"repository name prefix", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config-other/abc.tar.gz", false, ""},
		{"dot-dot in path", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/../../org-x/repo/abc.tar.gz", false, ""},
		{"encoded slash in path", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/..%2F..%2Forg-x%2Frepo%2Fabc.tar.gz", false, ""},
		{"directory only", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/", false, ""},
		{"query", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz?x=1", false, ""},
		{"empty query", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz?", false, ""},
		{"file scheme", "file:///gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false, ""},
		{"not a url", "://", false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := updater.validateArtifactURL(tc.url)
			if tc.valid && err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}
			if !tc.valid && err == nil {
				t.Fatalf("want error for %q, got nil", tc.url)
			}
			want := tc.want
			if tc.valid && want == "" {
				want = tc.url
			}
			if got != want {
				t.Fatalf("want URL %q, got %q", want, got)
			}
		})
	}
}

func TestNew_InvalidGitRepository(t *testing.T) {
	for _, gitRepository := range []string{"giantswarm-config", "a/b/c", "/name", "namespace/", "../name", "namespace/..", "Namespace/name", "namespace/na%2Fme", "namespace/name?x"} {
		_, err := New(Config{
			CacheDir:      t.TempDir(),
			ApiServerHost: testApiServerHost,
			ApiServerPort: "443",
			GitRepository: gitRepository,
		})
		if err == nil {
			t.Fatalf("want error for gitRepository %q, got nil", gitRepository)
		}
	}
}

func TestRunner_updateConfig_untrustedArtifactURL(t *testing.T) {
	archive, err := os.ReadFile("testdata/latestrevision.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	const (
		artifactPath = "/gitrepository/flux-giantswarm/giantswarm-config/latestrevision.tar.gz"
		redirectPath = "/gitrepository/flux-giantswarm/giantswarm-config/redirect.tar.gz"
		brokenPath   = "/gitrepository/flux-giantswarm/giantswarm-config/broken.tar.gz"
	)

	var foreignRequests atomic.Int32
	foreignServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignRequests.Add(1)
		w.Header().Add("Last-Modified", advertisedTimestamp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}))
	defer foreignServer.Close()

	foreignUrl, err := url.Parse(foreignServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	// Same listener, different host name: 127.0.0.1 is allowed below,
	// localhost is not.
	foreignBase := fmt.Sprintf("http://localhost:%s", foreignUrl.Port())

	srcCtrlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case artifactPath:
			w.Header().Add("Last-Modified", advertisedTimestamp)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archive)
		case redirectPath:
			http.Redirect(w, r, foreignBase+artifactPath, http.StatusFound)
		case brokenPath:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer srcCtrlServer.Close()

	srcCtrlUrl, err := url.Parse(srcCtrlServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	var advertisedURL atomic.Value
	k8sServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testGitRepositoryAPIPath {
			http.NotFoundHandler().ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":{"artifact":{"url":%q}}}`, advertisedURL.Load().(string))
	}))
	defer k8sServer.Close()

	k8sUrl, err := url.Parse(k8sServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name          string
		advertisedURL string
		cachedURL     string
		expectedError string
	}{
		{
			name:          "advertised URL on a foreign host",
			advertisedURL: foreignBase + artifactPath,
			expectedError: "not an allowed Source Controller host",
		},
		{
			name:          "advertised URL for another GitRepository",
			advertisedURL: srcCtrlServer.URL + "/gitrepository/org-x/repo/latestrevision.tar.gz",
			expectedError: "is not below",
		},
		{
			name:          "advertised URL redirecting to a foreign host",
			advertisedURL: srcCtrlServer.URL + redirectPath,
			expectedError: "expected 200, got 302",
		},
		{
			name:          "advertised URL with path traversal",
			advertisedURL: srcCtrlServer.URL + "/gitrepository/flux-giantswarm/giantswarm-config/../../org-x/repo/latestrevision.tar.gz",
			expectedError: "is not below",
		},
		{
			name:          "cached URL on a foreign host is dropped",
			advertisedURL: srcCtrlServer.URL + artifactPath,
			cachedURL:     foreignBase + artifactPath,
		},
		{
			name:          "cached URL redirecting to a foreign host is dropped",
			advertisedURL: srcCtrlServer.URL + artifactPath,
			cachedURL:     srcCtrlServer.URL + redirectPath,
		},
		{
			name:          "cached URL failing with a server error is dropped",
			advertisedURL: srcCtrlServer.URL + artifactPath,
			cachedURL:     srcCtrlServer.URL + brokenPath,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			foreignRequests.Store(0)
			advertisedURL.Store(tc.advertisedURL)

			cacheDir := t.TempDir()
			if tc.cachedURL != "" {
				err := prePopulateCache(cacheDir, []byte("latestrevision.tar.gz"), []byte("oldvalue"), []byte(advertisedTimestamp), []byte(tc.cachedURL))
				if err != nil {
					t.Fatal(err)
				}
			}

			updater, err := New(Config{
				CacheDir:            cacheDir,
				ApiServerHost:       k8sUrl.Hostname(),
				ApiServerPort:       k8sUrl.Port(),
				KubernetesTokenFile: testTokenFile,
				GitRepository:       testGitRepository,
				ArtifactHosts:       []string{srcCtrlUrl.Hostname()},
			})
			if err != nil {
				t.Fatal(err)
			}

			err = updater.UpdateConfig()

			if foreignRequests.Load() != 0 {
				t.Fatalf("foreign host was contacted %d times", foreignRequests.Load())
			}

			if tc.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.expectedError) {
					t.Fatalf("want error containing %q, got %v", tc.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}

			config, err := os.ReadFile(path.Join(cacheDir, "latest", "config.yaml")) //nolint:gosec
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(config)) != newConfigValue {
				t.Fatalf("want %q, got %q", newConfigValue, string(config))
			}

			cached, err := os.ReadFile(path.Join(cacheDir, cacheLastArtifactUrl)) //nolint:gosec
			if err != nil {
				t.Fatal(err)
			}
			if string(cached) != tc.advertisedURL {
				t.Fatalf("want cached URL %q, got %q", tc.advertisedURL, string(cached))
			}
		})
	}
}

const (
	defaultHost      = "source-controller.flux-giantswarm.svc.cluster.local"
	customDomainHost = "source-controller.flux-giantswarm.svc.example.internal"
	fluxSystemHost   = "source-controller.flux-system.svc.cluster.local"
	errClusterDomain = "clusterDomain"
)

func TestArtifactHost(t *testing.T) {
	testCases := []struct {
		name          string
		namespace     string
		clusterDomain string
		want          string
	}{
		{name: "default", namespace: DefaultSourceControllerNamespace, clusterDomain: "cluster.local", want: defaultHost},
		{name: "custom domain with trailing dot", namespace: DefaultSourceControllerNamespace, clusterDomain: "example.internal.", want: customDomainHost},
		{name: "other namespace", namespace: "flux-system", clusterDomain: "cluster.local", want: fluxSystemHost},
		{name: "empty domain", namespace: DefaultSourceControllerNamespace, clusterDomain: "", want: defaultHost},
		{name: "dot as domain", namespace: DefaultSourceControllerNamespace, clusterDomain: ".", want: defaultHost},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ArtifactHost(tc.namespace, tc.clusterDomain); got != tc.want {
				t.Fatalf("ArtifactHost(%q, %q): want %q, got %q", tc.namespace, tc.clusterDomain, tc.want, got)
			}
			if tc.namespace == DefaultSourceControllerNamespace {
				if got := DefaultArtifactHosts(tc.clusterDomain); !reflect.DeepEqual(got, []string{tc.want}) {
					t.Fatalf("DefaultArtifactHosts(%q): want [%s], got %v", tc.clusterDomain, tc.want, got)
				}
			}
		})
	}
}

func TestNew_ClusterDomain(t *testing.T) {
	testCases := []struct {
		name          string
		clusterDomain string
		artifactHosts []string
		want          []string
		wantError     string
	}{
		{name: "empty domain", want: []string{defaultHost}},
		{name: "custom domain", clusterDomain: "example.internal", want: []string{customDomainHost}},
		{name: "artifact hosts win over the domain", clusterDomain: "example.internal", artifactHosts: []string{fluxSystemHost}, want: []string{fluxSystemHost}},
		{name: "domain with port", clusterDomain: "cluster.local:443", wantError: errClusterDomain},
		{name: "domain with path", clusterDomain: "cluster.local/x", wantError: errClusterDomain},
		{name: "domain with user info", clusterDomain: "user@cluster.local", wantError: errClusterDomain},
		{name: "domain with empty label", clusterDomain: "cluster..local", wantError: errClusterDomain},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updater, err := New(Config{
				CacheDir:      t.TempDir(),
				ApiServerHost: testApiServerHost,
				ApiServerPort: "443",
				GitRepository: testGitRepository,
				ArtifactHosts: tc.artifactHosts,
				ClusterDomain: tc.clusterDomain,
			})

			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("want error containing %q, got %v", tc.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(updater.ArtifactHosts, tc.want) {
				t.Fatalf("want artifact hosts %v, got %v", tc.want, updater.ArtifactHosts)
			}
		})
	}
}

func TestRunner_updateConfig_artifactTooLarge(t *testing.T) {
	archive, err := os.ReadFile("testdata/latestrevision.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	defer func(limit int64) { maxArtifactSize = limit }(maxArtifactSize)

	const artifactPath = "/gitrepository/flux-giantswarm/giantswarm-config/latestrevision.tar.gz"

	var announceOnly atomic.Bool
	srcCtrlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Last-Modified", advertisedTimestamp)
		if announceOnly.Load() {
			// Announce a large body and send none of it: only the
			// Content-Length check can refuse this without reading.
			w.Header().Set("Content-Length", strconv.FormatInt(maxArtifactSize+1, 10))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}))
	defer srcCtrlServer.Close()

	srcCtrlUrl, err := url.Parse(srcCtrlServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	k8sServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":{"artifact":{"url":%q}}}`, srcCtrlServer.URL+artifactPath)
	}))
	defer k8sServer.Close()

	k8sUrl, err := url.Parse(k8sServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name         string
		limit        int64
		announceOnly bool
		wantError    bool
	}{
		{name: "one byte over the limit", limit: int64(len(archive)) - 1, wantError: true},
		{name: "exactly at the limit", limit: int64(len(archive))},
		{name: "announced size over the limit", limit: int64(len(archive)), announceOnly: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			maxArtifactSize = tc.limit
			announceOnly.Store(tc.announceOnly)

			cacheDir := t.TempDir()
			if err := prePopulateCache(cacheDir, []byte("oldrevision.tar.gz"), []byte("oldvalue"), []byte(advertisedTimestamp), []byte("")); err != nil {
				t.Fatal(err)
			}

			updater, err := New(Config{
				CacheDir:            cacheDir,
				ApiServerHost:       k8sUrl.Hostname(),
				ApiServerPort:       k8sUrl.Port(),
				KubernetesTokenFile: testTokenFile,
				GitRepository:       testGitRepository,
				ArtifactHosts:       []string{srcCtrlUrl.Hostname()},
			})
			if err != nil {
				t.Fatal(err)
			}

			err = updater.UpdateConfig()

			want := newConfigValue
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "artifact is larger than") {
					t.Fatalf("want size limit error, got %v", err)
				}
				// The previous config stays in place when the download is refused.
				want = "oldvalue"
			} else if err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}

			config, err := os.ReadFile(path.Join(cacheDir, "latest", "config.yaml")) //nolint:gosec
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(config)) != want {
				t.Fatalf("want config %q, got %q", want, string(config))
			}
		})
	}
}

func TestRunner_updateConfig_brokenArchiveIsRetried(t *testing.T) {
	const artifactPath = "/gitrepository/flux-giantswarm/giantswarm-config/broken.tar.gz"

	srcCtrlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Last-Modified", advertisedTimestamp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not a tar archive"))
	}))
	defer srcCtrlServer.Close()

	srcCtrlUrl, err := url.Parse(srcCtrlServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	k8sServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":{"artifact":{"url":%q}}}`, srcCtrlServer.URL+artifactPath)
	}))
	defer k8sServer.Close()

	k8sUrl, err := url.Parse(k8sServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	cacheDir := t.TempDir()
	if err := prePopulateCache(cacheDir, []byte("oldrevision.tar.gz"), []byte("oldvalue"), []byte(advertisedTimestamp), []byte("")); err != nil {
		t.Fatal(err)
	}

	updater, err := New(Config{
		CacheDir:            cacheDir,
		ApiServerHost:       k8sUrl.Hostname(),
		ApiServerPort:       k8sUrl.Port(),
		KubernetesTokenFile: testTokenFile,
		GitRepository:       testGitRepository,
		ArtifactHosts:       []string{srcCtrlUrl.Hostname()},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := updater.UpdateConfig(); err == nil {
		t.Fatal("want error for a broken archive, got nil")
	}

	// The cached artifact is forgotten, so the next update downloads it
	// again instead of keeping a partial config.
	for _, name := range []string{cacheLastArtifactUrl, cacheLastArchiveTimestamp, cacheLastArchive} {
		if _, err := os.Stat(path.Join(cacheDir, name)); !os.IsNotExist(err) {
			t.Fatalf("want %s to be removed, got %v", name, err)
		}
	}
}

func TestNew_InvalidArtifactHosts(t *testing.T) {
	for _, host := range []string{"", "source-controller:80", "http://source-controller", "source-controller/path", "[::1]", ArtifactHost("", "cluster.local"), "Source_Controller.svc", "source-controller", "source-controller.flux-giantswarm.svc"} {
		_, err := New(Config{
			CacheDir:      t.TempDir(),
			ApiServerHost: testApiServerHost,
			ApiServerPort: "443",
			GitRepository: testGitRepository,
			ArtifactHosts: []string{host},
		})
		if err == nil {
			t.Fatalf("want error for artifact host %q, got nil", host)
		}
	}
}

func TestRunner_updateConfig_failedDownloadsDoNotLeak(t *testing.T) {
	srcCtrlServer := httptest.NewServer(http.NotFoundHandler())
	defer srcCtrlServer.Close()

	k8sServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testGitRepositoryAPIPath {
			http.NotFoundHandler().ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":{"artifact":{"url":%q}}}`, srcCtrlServer.URL+"/gitrepository/flux-giantswarm/giantswarm-config/missing.tar.gz")
	}))
	defer k8sServer.Close()

	srcCtrlUrl, err := url.Parse(srcCtrlServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	k8sUrl, err := url.Parse(k8sServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	updater, err := New(Config{
		CacheDir:            t.TempDir(),
		ApiServerHost:       k8sUrl.Hostname(),
		ApiServerPort:       k8sUrl.Port(),
		KubernetesTokenFile: testTokenFile,
		GitRepository:       testGitRepository,
		ArtifactHosts:       []string{srcCtrlUrl.Hostname()},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Warm up connection pools before taking the baseline.
	_ = updater.UpdateConfig()
	baseline := runtime.NumGoroutine()

	for range 50 {
		if err := updater.UpdateConfig(); err == nil {
			t.Fatal("want error for a missing artifact, got nil")
		}
	}

	// Allow closed connections to wind down.
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > baseline+10 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}

	if got := runtime.NumGoroutine(); got > baseline+10 {
		t.Fatalf("goroutines grew from %d to %d after 50 failed updates", baseline, got)
	}
}

func TestValidateArtifactURL_shortHostUsesClusterDomain(t *testing.T) {
	const (
		customDomain = "example.internal"
		want         = "http://" + customDomainHost + "/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz"
	)

	for _, tc := range []struct {
		name          string
		clusterDomain string
		artifactHosts []string
		want          string
	}{
		{"custom cluster domain", customDomain, nil, want},
		{"explicit hosts", customDomain, []string{customDomainHost}, want},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updater, err := New(Config{
				CacheDir:      t.TempDir(),
				ApiServerHost: testApiServerHost,
				ApiServerPort: "443",
				GitRepository: testGitRepository,
				ArtifactHosts: tc.artifactHosts,
				ClusterDomain: tc.clusterDomain,
			})
			if err != nil {
				t.Fatalf("want nil, got error: %s", err)
			}

			got, err := updater.validateArtifactURL("http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz")
			if err != nil {
				t.Fatalf("want nil, got error: %s", err)
			}
			if got != tc.want {
				t.Fatalf("want URL %q, got %q", tc.want, got)
			}

			if _, err := updater.validateArtifactURL("http://source-controller.flux-giantswarm.svc.cluster.local/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz"); err == nil {
				t.Fatal("want error for a host with the default cluster domain, got nil")
			}
		})
	}
}

// A short advertised host is completed before anything is requested: the HEAD
// for a cached URL, the download and the cached URL all use the completed host.
func TestRunner_updateConfig_shortHostIsCompleted(t *testing.T) {
	archive, err := os.ReadFile("testdata/latestrevision.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	const (
		artifactPath  = "/gitrepository/flux-giantswarm/giantswarm-config/latestrevision.tar.gz"
		shortURL      = "http://source-controller.flux-giantswarm.svc" + artifactPath
		completedHost = "source-controller.flux-giantswarm.svc.cluster.local"
		completedURL  = "http://" + completedHost + artifactPath
	)

	srcCtrlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != artifactPath {
			http.NotFoundHandler().ServeHTTP(w, r)
			return
		}
		w.Header().Add("Last-Modified", advertisedTimestamp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}))
	defer srcCtrlServer.Close()

	srcCtrlUrl, err := url.Parse(srcCtrlServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	k8sServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testGitRepositoryAPIPath {
			http.NotFoundHandler().ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":{"artifact":{"url":%q}}}`, shortURL)
	}))
	defer k8sServer.Close()

	k8sUrl, err := url.Parse(k8sServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		cachedURL string
		want      []string
	}{
		{name: "advertised short URL", want: []string{"GET " + completedHost}},
		// A short URL cached by konfigure before v2.2.0. Its timestamp is older
		// than the artifact, so the HEAD is followed by a download.
		{name: "cached short URL", cachedURL: shortURL, want: []string{"HEAD " + completedHost, "GET " + completedHost}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			if tc.cachedURL != "" {
				err := prePopulateCache(cacheDir, []byte("latestrevision.tar.gz"), []byte("oldvalue"), []byte(time.Time{}.Format(http.TimeFormat)), []byte(tc.cachedURL))
				if err != nil {
					t.Fatal(err)
				}
			}

			updater, err := New(Config{
				CacheDir:            cacheDir,
				ApiServerHost:       k8sUrl.Hostname(),
				ApiServerPort:       k8sUrl.Port(),
				KubernetesTokenFile: testTokenFile,
				GitRepository:       testGitRepository,
			})
			if err != nil {
				t.Fatal(err)
			}

			var requests []string
			updater.artifactTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.Method+" "+r.URL.Host)
				forwarded := r.Clone(r.Context())
				forwarded.URL.Host = srcCtrlUrl.Host
				forwarded.Host = ""
				return http.DefaultTransport.RoundTrip(forwarded)
			})

			if err := updater.UpdateConfig(); err != nil {
				t.Fatalf("want nil, got error: %s", err)
			}

			if !reflect.DeepEqual(requests, tc.want) {
				t.Fatalf("want requests %v, got %v", tc.want, requests)
			}

			cached, err := os.ReadFile(path.Join(cacheDir, cacheLastArtifactUrl)) //nolint:gosec
			if err != nil {
				t.Fatal(err)
			}
			if string(cached) != completedURL {
				t.Fatalf("want cached URL %q, got %q", completedURL, string(cached))
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestValidateArtifactURL_errorNamesAdvertisedHost(t *testing.T) {
	updater, err := New(Config{
		CacheDir:      t.TempDir(),
		ApiServerHost: testApiServerHost,
		ApiServerPort: "443",
		GitRepository: testGitRepository,
	})
	if err != nil {
		t.Fatalf("want nil, got error: %s", err)
	}

	_, err = updater.validateArtifactURL("http://source-controller.flux-system.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz")
	if err == nil || !strings.Contains(err.Error(), `"source-controller.flux-system.svc"`) {
		t.Fatalf("want error naming the advertised host, got %v", err)
	}
}
