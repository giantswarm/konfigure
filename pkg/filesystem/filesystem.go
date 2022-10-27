package filesystem

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/giantswarm/microerror"
)

type Store struct {
	Dir string
}

func (s *Store) ReadFile(path string) ([]byte, error) {
	if filepath.IsAbs(path) {
		panic(fmt.Sprintf(
			"%q is an absolute path; expected sub-path of %q",
			path, s.Dir,
		))
	}
	return os.ReadFile(filepath.Join(s.Dir, path))
}

func (s *Store) ReadDir(path string) ([]fs.DirEntry, error) {
	if filepath.IsAbs(path) {
		panic(fmt.Sprintf(
			"%q is an absolute path; expected sub-path of %q",
			path, s.Dir,
		))
	}
	return os.ReadDir(filepath.Join(s.Dir, path))
}

// Version returns version of config files contained in Store.Dir. The
// directory is expected to be a git repository. Returned version uses a
// tag-like format: `v10.2.0` for tags, `v10.2.0-27-gf4262c857` for commits.
func (s *Store) Version() (string, error) {
	cmd := exec.Command("git", "describe", "--tags")
	cmd.Dir = s.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", microerror.Mask(err)
	}
	return strings.TrimSpace(string(out)), nil
}
