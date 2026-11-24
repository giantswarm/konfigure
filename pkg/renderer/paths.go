package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// joinInDir joins elems onto dir and returns the cleaned result. It fails
// when an element is an absolute path or when the joined path, after
// cleaning, is outside of dir.
func joinInDir(dir string, elems ...string) (string, error) {
	for _, elem := range elems {
		if filepath.IsAbs(elem) || filepath.VolumeName(elem) != "" {
			return "", fmt.Errorf("path %q must be relative to the config directory", elem)
		}
	}

	root := filepath.Clean(dir)
	joined := filepath.Join(append([]string{root}, elems...)...)

	if !isWithin(root, joined) {
		return "", fmt.Errorf("path %q is outside of the config directory", strings.Join(elems, "/"))
	}

	return joined, nil
}

// resolveInDir resolves symbolic links in p, which must already be inside
// dir lexically, and returns the resolved path. It fails when the resolved
// path is outside of dir after resolving symbolic links in dir as well.
// Errors from resolving p, such as fs.ErrNotExist, are returned unchanged so
// callers can tell a missing path from a refused one.
func resolveInDir(dir, p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}

	root, err := filepath.EvalSymlinks(filepath.Clean(dir))
	if err != nil {
		return "", err
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}

	if !isWithin(absRoot, absResolved) {
		rel, relErr := filepath.Rel(filepath.Clean(dir), p)
		if relErr != nil {
			rel = p
		}
		return "", fmt.Errorf("path %q resolves to a location outside of the config directory", rel)
	}

	return resolved, nil
}

// isWithin reports whether p is root or a path below root. Both paths must be
// clean and either both absolute or both relative to the same directory.
func isWithin(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}
