package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// relInDir joins elems into a clean path relative to the config directory.
// Leading separators are dropped, so "/default" means "default" inside the
// config directory, as it always did. It fails when an element has a volume
// name or when the joined path leaves the config directory after cleaning.
func relInDir(elems ...string) (string, error) {
	trimmed := make([]string, 0, len(elems))

	for _, elem := range elems {
		if filepath.VolumeName(elem) != "" {
			return "", fmt.Errorf("path %q must be relative to the config directory", elem)
		}
		trimmed = append(trimmed, strings.TrimLeftFunc(elem, func(r rune) bool { return r == '/' || r == filepath.Separator }))
	}

	rel := filepath.Clean(filepath.Join(trimmed...))
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside of the config directory", strings.Join(elems, "/"))
	}

	return rel, nil
}

// wrapRootError keeps "does not exist" errors as they are, so callers can
// tell a missing optional file from a refused one, and adds context to all
// other errors, including os.Root refusing a symbolic link that leaves the
// config directory.
func wrapRootError(rel string, err error) error {
	if err == nil || os.IsNotExist(err) {
		return err
	}

	return fmt.Errorf("path %q could not be read inside the config directory: %w", rel, err)
}

// readFileInDir reads the file at rel inside dir. Symbolic links are followed
// only while they stay inside dir.
func readFileInDir(dir, rel string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

	content, err := root.ReadFile(rel)

	return content, wrapRootError(rel, err)
}
