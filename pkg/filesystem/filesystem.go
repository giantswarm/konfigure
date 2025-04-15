package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	filePath := filepath.Join(s.Dir, path)
	filePath = filepath.Clean(filePath)
	return os.ReadFile(filePath)
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
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 128 {
		// This handles the case when konfigure is being run locally on a shallow copy:
		//
		//    $ git describe --tags
		//    fatal: No names found, cannot describe anything.
		//    $ echo $?
		//    128
		//
		cmd := exec.Command("git", "rev-parse", "--short=10", "HEAD")
		cmd.Dir = s.Dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", err
		}
		return "v0.0.0-" + strings.TrimSpace(string(out)), nil
	} else if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
