package filesystem

import (
	"io/ioutil"
	"os"
)

type Store struct {
	Dir string
}

func (s *Store) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (s *Store) ReadDir(path string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(path)
}
