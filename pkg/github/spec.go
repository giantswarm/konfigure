package github

import (
	"os"
)

type Store interface {
	// ReadFile is similar to io/ioutil.ReadFile but it returns error
	// matched by IsNotFound if the file does not exist.
	ReadFile(path string) ([]byte, error)
	// ReadDir is similar to io/ioutil.ReadDir but it returns error matched
	// by IsNotFound if the directory does not exist.
	ReadDir(dirname string) ([]os.FileInfo, error)
}
