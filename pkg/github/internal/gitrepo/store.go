package gitrepo

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/giantswarm/microerror"
	"github.com/go-git/go-billy/v5"
)

type Store struct {
	fs billy.Filesystem
}

func (s *Store) GetContent(ctx context.Context, path string) ([]byte, error) {
	stat, err := s.fs.Stat(path)
	if os.IsNotExist(err) {
		return nil, microerror.Maskf(executionFailedError, "file %#q does not exist", path)
	} else if err != nil {
		return nil, microerror.Mask(err)
	}
	if stat.IsDir() {
		return nil, microerror.Maskf(executionFailedError, "file %#q is a directory", path)
	}

	f, err := s.fs.Open(path)
	if os.IsNotExist(err) {
		return nil, microerror.Maskf(executionFailedError, "file %#q does not exist", path)
	} else if err != nil {
		return nil, microerror.Mask(err)
	}

	bs, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return bs, nil
}
