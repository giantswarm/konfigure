package generator

import (
	"context"
	"os"
)

type Filesystem interface {
	ReadFile(string) ([]byte, error)
	ReadDir(string) ([]os.FileInfo, error)
}

type DecryptTraverser interface {
	Traverse(context.Context, []byte) ([]byte, error)
}
