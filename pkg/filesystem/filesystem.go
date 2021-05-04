package filesystem

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/giantswarm/microerror"
)

type Store struct {
	Dir string
}

func (s *Store) ReadFile(path string) ([]byte, error) {
	if filepath.IsAbs(path) {
		return []byte{}, microerror.Maskf(
			invalidPathError,
			"%q is an absolute path; expected sub-path of %q",
			path, s.Dir,
		)
	}
	return os.ReadFile(filepath.Join(s.Dir, path))
}

func (s *Store) ReadDir(path string) ([]os.FileInfo, error) {
	if filepath.IsAbs(path) {
		return []byte{}, microerror.Maskf(
			invalidPathError,
			"%q is an absolute path; expected sub-path of %q",
			path, s.Dir,
		)
	}
	return ioutil.ReadDir(filepath.Join(s.Dir, path))
}
