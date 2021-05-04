package filesystem

import (
	"os"
)

type Store struct{}

func (s *Store) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)

}

func (s *Store) ReadDir(path string) ([]os.FileInfo, error) {
	return os.ReadDir(path)
}
