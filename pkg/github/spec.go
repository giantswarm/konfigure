package github

import "context"

type Store interface {
	GetContent(ctx context.Context, path string) ([]byte, error)
}
