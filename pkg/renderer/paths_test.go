package renderer

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giantswarm/konfigure/v2/pkg/model"
)

const (
	errOutside        = "outside of the config directory"
	errSymlinkOutside = "escapes from parent"
	errNotExist       = "does not exist"
	insideContent     = "inside: true\n"
	missingFile       = "missing.yaml"
	missingDir        = "missing"
	secretFile        = "secret.yaml"
	valuesFile        = "values.yaml"
	appsOperator      = "apps/operator"
	defaultDir        = "default"
	includeDir        = "include"
	snippetName       = "snippet"
)

// setupConfinementDirs creates a config directory and a sibling directory
// outside of it, each with one file, and returns both directories.
func setupConfinementDirs(t *testing.T) (configDir, outsideDir string) {
	t.Helper()

	base := t.TempDir()
	configDir = filepath.Join(base, "ns", "repo", "latest")
	outsideDir = filepath.Join(base, "other-ns", "other-repo", "latest")

	for _, dir := range []string{
		filepath.Join(configDir, defaultDir, "apps", "operator"),
		filepath.Join(configDir, includeDir),
		filepath.Join(outsideDir, defaultDir),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(t, filepath.Join(configDir, defaultDir, "apps", "operator", valuesFile), insideContent)
	writeFile(t, filepath.Join(configDir, includeDir, "snippet.tmpl"), "hello {{ .name }}")
	writeFile(t, filepath.Join(outsideDir, defaultDir, secretFile), "outside: true\n")
	writeFile(t, filepath.Join(outsideDir, "snippet.tmpl"), "outside")

	return configDir, outsideDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFileFromPathSegments_Confinement(t *testing.T) {
	configDir, outsideDir := setupConfinementDirs(t)

	if err := os.Symlink(filepath.Join(outsideDir, defaultDir), filepath.Join(configDir, "escape-dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outsideDir, defaultDir, secretFile), filepath.Join(configDir, defaultDir, "escape-file.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outsideDir, missingFile), filepath.Join(configDir, defaultDir, "escape-missing.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("apps", "operator", valuesFile), filepath.Join(configDir, defaultDir, "inside-link.yaml")); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name            string
		segments        []PathSegment
		expectedContent string
		expectedError   string
	}{
		{
			name: "file inside the config directory",
			segments: []PathSegment{
				{defaultDir, true},
				{appsOperator, true},
				{valuesFile, true},
			},
			expectedContent: insideContent,
		},
		{
			name: "dot-dot that stays inside the config directory",
			segments: []PathSegment{
				{"default/apps", true},
				{"../apps/operator", true},
				{valuesFile, true},
			},
			expectedContent: insideContent,
		},
		{
			name: "symbolic link that stays inside the config directory",
			segments: []PathSegment{
				{defaultDir, true},
				{"", false},
				{"inside-link.yaml", true},
			},
			expectedContent: insideContent,
		},
		{
			name: "leading slashes are relative to the config directory",
			segments: []PathSegment{
				{"/" + defaultDir, true},
				{"/" + appsOperator, true},
				{valuesFile, true},
			},
			expectedContent: insideContent,
		},
		{
			name: "missing optional file",
			segments: []PathSegment{
				{defaultDir, true},
				{appsOperator, true},
				{missingFile, false},
			},
			expectedContent: "",
		},
		{
			name: "missing required file",
			segments: []PathSegment{
				{defaultDir, true},
				{appsOperator, true},
				{missingFile, true},
			},
			expectedError: "does not exist",
		},
		{
			name: "dot-dot escaping into a sibling repository",
			segments: []PathSegment{
				{"../../../other-ns/other-repo/latest", true},
				{defaultDir, true},
				{secretFile, true},
			},
			expectedError: errOutside,
		},
		{
			name: "dot-dot escaping in the file name",
			segments: []PathSegment{
				{defaultDir, true},
				{"", false},
				{"../../../../other-ns/other-repo/latest/default/secret.yaml", true},
			},
			expectedError: errOutside,
		},
		{
			name: "dot-dot escaping in an optional segment",
			segments: []PathSegment{
				{defaultDir, false},
				{"../../..", false},
				{missingFile, false},
			},
			expectedError: errOutside,
		},
		{
			name: "absolute path in the first segment stays relative",
			segments: []PathSegment{
				{filepath.Join(outsideDir, defaultDir), true},
				{"", false},
				{secretFile, true},
			},
			expectedError: errNotExist,
		},
		{
			name: "absolute path in the file name stays relative",
			segments: []PathSegment{
				{defaultDir, true},
				{"", false},
				{filepath.Join(outsideDir, defaultDir, secretFile), true},
			},
			expectedError: errNotExist,
		},
		{
			name: "symbolic link to a directory outside",
			segments: []PathSegment{
				{"escape-dir", true},
				{"", false},
				{secretFile, true},
			},
			expectedError: errSymlinkOutside,
		},
		{
			name: "optional symbolic link to a missing file outside",
			segments: []PathSegment{
				{defaultDir, true},
				{"", false},
				{"escape-missing.yaml", false},
			},
			expectedError: errSymlinkOutside,
		},
		{
			name: "symbolic link to a file outside",
			segments: []PathSegment{
				{defaultDir, true},
				{"", false},
				{"escape-file.yaml", true},
			},
			expectedError: errSymlinkOutside,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := loadFileFromPathSegments(configDir, tc.segments)

			if tc.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got content %q", tc.expectedError, string(content))
				}
				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Fatalf("expected error containing %q, got %q", tc.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != tc.expectedContent {
				t.Fatalf("expected content %q, got %q", tc.expectedContent, string(content))
			}
		})
	}
}

func TestLoadValueFiles_VariableCannotEscape(t *testing.T) {
	configDir, _ := setupConfinementDirs(t)

	schema := &model.Schema{
		Layers: []model.Layer{
			{
				Id:   defaultDir,
				Path: model.Path{Directory: defaultDir, Required: true},
				Values: model.Values{
					Path:      model.Path{Directory: "apps/<< app >>", Required: true},
					ConfigMap: model.Value{Name: valuesFile, Required: true},
				},
			},
		},
	}

	_, err := LoadValueFiles(configDir, schema, SchemaVariables{"app": "../../../../../other-ns/other-repo/latest/default"})
	if err == nil || !strings.Contains(err.Error(), errOutside) {
		t.Fatalf("expected confinement error, got %v", err)
	}

	valueFiles, err := LoadValueFiles(configDir, schema, SchemaVariables{"app": "operator"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valueFiles.ConfigMaps[defaultDir] != insideContent {
		t.Fatalf("unexpected value file content %q", valueFiles.ConfigMaps[defaultDir])
	}
}

func TestIncludeFunction_Confinement(t *testing.T) {
	configDir, outsideDir := setupConfinementDirs(t)

	if err := os.Symlink(filepath.Join(outsideDir, "snippet.tmpl"), filepath.Join(configDir, includeDir, "escape.tmpl")); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name           string
		directory      string
		templateName   string
		expected       string
		expectedError  string
		expectNotExist bool
	}{
		{
			name:         "template inside the include directory",
			directory:    includeDir,
			templateName: snippetName,
			expected:     "hello world",
		},
		{
			name:          "template name escaping the config directory",
			directory:     includeDir,
			templateName:  "../../../../other-ns/other-repo/latest/snippet",
			expectedError: errOutside,
		},
		{
			name:          "include directory escaping the config directory",
			directory:     "../../../other-ns/other-repo/latest",
			templateName:  snippetName,
			expectedError: errOutside,
		},
		{
			name:           "absolute include directory stays relative",
			directory:      outsideDir,
			templateName:   snippetName,
			expectNotExist: true,
		},
		{
			name:           "absolute template name stays relative",
			directory:      includeDir,
			templateName:   filepath.Join(outsideDir, snippetName),
			expectNotExist: true,
		},
		{
			name:          "symbolic link to a template outside",
			directory:     includeDir,
			templateName:  "escape",
			expectedError: errSymlinkOutside,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			include := model.Include{
				Function:  model.IncludeFunction{Name: includeDir},
				Path:      model.Path{Directory: tc.directory},
				Extension: ".tmpl",
			}

			out, err := generateIncludeFunction(configDir, include)(tc.templateName, map[string]string{"name": "world"})

			if tc.expectNotExist {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("want a not-exist error, got %v (output %q)", err, out)
				}
				return
			}

			if tc.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got output %q", tc.expectedError, out)
				}
				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Fatalf("expected error containing %q, got %q", tc.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, out)
			}
		})
	}
}

func TestLoadFileFromPathSegments_DirForms(t *testing.T) {
	configDir, _ := setupConfinementDirs(t)
	segments := []PathSegment{{defaultDir, true}, {appsOperator, true}, {valuesFile, true}}

	t.Run("symbolic link as config directory", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "config")
		if err := os.Symlink(configDir, link); err != nil {
			t.Fatal(err)
		}

		content, err := loadFileFromPathSegments(link, segments)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(content) != insideContent {
			t.Fatalf("expected content %q, got %q", insideContent, string(content))
		}
	})

	t.Run("relative config directory", func(t *testing.T) {
		t.Chdir(configDir)

		content, err := loadFileFromPathSegments(".", segments)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(content) != insideContent {
			t.Fatalf("expected content %q, got %q", insideContent, string(content))
		}
	})

	t.Run("missing config directory with a required segment", func(t *testing.T) {
		_, err := loadFileFromPathSegments(filepath.Join(configDir, missingDir), []PathSegment{{defaultDir, true}})
		if err == nil || !strings.Contains(err.Error(), errNotExist) {
			t.Fatalf("want error containing %q, got %v", errNotExist, err)
		}
	})

	t.Run("missing config directory with optional segments", func(t *testing.T) {
		content, err := loadFileFromPathSegments(filepath.Join(configDir, missingDir), []PathSegment{{defaultDir, false}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(content) != 0 {
			t.Fatalf("expected empty content, got %q", string(content))
		}
	})
}
