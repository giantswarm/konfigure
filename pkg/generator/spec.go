package generator

import (
	"context"
	"io/fs"
)

type Filesystem interface {
	ReadFile(string) ([]byte, error)
	ReadDir(string) ([]fs.DirEntry, error)
}

type DecryptTraverser interface {
	Traverse(context.Context, []byte) ([]byte, error)
}
