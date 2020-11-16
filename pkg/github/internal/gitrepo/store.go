package gitrepo

import (
	"io/ioutil"
	"os"

	"github.com/giantswarm/microerror"
	"github.com/go-git/go-billy/v5"
)

type Store struct {
	fs billy.Filesystem
}

func (s *Store) ReadDir(dirpath string) ([]os.FileInfo, error) {
	stat, err := s.fs.Stat(dirpath)
	if os.IsNotExist(err) {
		return nil, microerror.Maskf(notFoundError, "file %#q does not exist", dirpath)
	} else if err != nil {
		return nil, microerror.Mask(err)
	}
	if !stat.IsDir() {
		return nil, microerror.Maskf(executionFailedError, "file %#q is not a directory", dirpath)
	}

	fs, err := s.fs.ReadDir(dirpath)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return fs, nil
}

func (s *Store) ReadFile(path string) ([]byte, error) {
	stat, err := s.fs.Stat(path)
	if os.IsNotExist(err) {
		return nil, microerror.Maskf(notFoundError, "file %#q does not exist", path)
	} else if err != nil {
		return nil, microerror.Mask(err)
	}
	if stat.IsDir() {
		return nil, microerror.Maskf(executionFailedError, "file %#q is a directory", path)
	}

	f, err := s.fs.Open(path)
	if os.IsNotExist(err) {
		return nil, microerror.Maskf(notFoundError, "file %#q does not exist", path)
	} else if err != nil {
		return nil, microerror.Mask(err)
	}
	defer f.Close()

	bs, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return bs, nil
}
