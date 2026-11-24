package fluxupdater

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"runtime"
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
	}{
		{"default host", "http://source-controller.flux-giantswarm.svc.cluster.local/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true},
		{"default host with trailing dot", "http://source-controller.flux-giantswarm.svc.cluster.local./gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true},
		{"default host with port", "http://source-controller.flux-giantswarm.svc.cluster.local:80/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true},
		{"https", "https://source-controller.flux-giantswarm.svc.cluster.local/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", true},
		{"short service name", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false},
		{"other cluster domain", "http://source-controller.flux-giantswarm.svc.example.internal/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false},
		{"foreign host", "http://other.example.com/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false},
		{"source controller in another namespace", "http://source-controller.flux-system.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false},
		{"ip address", "http://10.0.0.1/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false},
		{"user info", "http://source-controller.flux-giantswarm.svc@other.example.com/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false},
		{"other repository", "http://source-controller.flux-giantswarm.svc/gitrepository/org-x/repo/abc.tar.gz", false},
		{"repository name prefix", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config-other/abc.tar.gz", false},
		{"dot-dot in path", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/../../org-x/repo/abc.tar.gz", false},
		{"encoded slash in path", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/..%2F..%2Forg-x%2Frepo%2Fabc.tar.gz", false},
		{"directory only", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/", false},
		{"query", "http://source-controller.flux-giantswarm.svc/gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz?x=1", false},
		{"file scheme", "file:///gitrepository/flux-giantswarm/giantswarm-config/abc.tar.gz", false},
		{"not a url", "://", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := updater.validateArtifactURL(tc.url)
			if tc.valid && err != nil {
				t.Fatalf("want nil, got error: %s", err.Error())
			}
			if !tc.valid && err == nil {
				t.Fatalf("want error for %q, got nil", tc.url)
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

func TestArtifactHost(t *testing.T) {
	testCases := map[string][2]string{
		"source-controller.flux-giantswarm.svc.cluster.local":    {"flux-giantswarm", "cluster.local"},
		"source-controller.flux-giantswarm.svc.example.internal": {"flux-giantswarm", "example.internal."},
		"source-controller.flux-system.svc.cluster.local":        {"flux-system", "cluster.local"},
	}

	for want, args := range testCases {
		if got := ArtifactHost(args[0], args[1]); got != want {
			t.Fatalf("ArtifactHost(%q, %q): want %q, got %q", args[0], args[1], want, got)
		}
	}

	if got := DefaultArtifactHosts(); len(got) != 1 || got[0] != "source-controller.flux-giantswarm.svc.cluster.local" {
		t.Fatalf("unexpected default artifact hosts %v", got)
	}
}

func TestRunner_updateConfig_artifactTooLarge(t *testing.T) {
	archive, err := os.ReadFile("testdata/latestrevision.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	defer func(limit int64) { maxArtifactSize = limit }(maxArtifactSize)
	maxArtifactSize = int64(len(archive)) - 1

	const artifactPath = "/gitrepository/flux-giantswarm/giantswarm-config/latestrevision.tar.gz"

	srcCtrlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	err = updater.UpdateConfig()
	if err == nil || !strings.Contains(err.Error(), "artifact is larger than") {
		t.Fatalf("want size limit error, got %v", err)
	}

	// The previous config stays in place when the download is refused.
	config, err := os.ReadFile(path.Join(cacheDir, "latest", "config.yaml")) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	if string(config) != "oldvalue" {
		t.Fatalf("want the old config to stay, got %q", string(config))
	}
}

func TestNew_InvalidArtifactHosts(t *testing.T) {
	for _, host := range []string{"", "source-controller:80", "http://source-controller", "source-controller/path", "[::1]"} {
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
