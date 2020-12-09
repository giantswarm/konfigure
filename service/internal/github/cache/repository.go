package cache

import (
	"context"
	"fmt"

	gocache "github.com/patrickmn/go-cache"

	"github.com/giantswarm/config-controller/pkg/github"
)

type Repository struct {
	underlying *gocache.Cache
}

func NewRepository() *Repository {
	return &Repository{
		underlying: gocache.New(expiration, expiration/2),
	}
}

func (r *Repository) Get(ctx context.Context, key string) (github.Store, bool) {
	val, ok := r.underlying.Get(key)
	if ok {
		return val.(github.Store), ok
	}

	return nil, false
}

func (r *Repository) Set(ctx context.Context, key string, value github.Store) {
	r.underlying.SetDefault(key, value)
}

func (r *Repository) Key(owner, name, reference string) string {
	return fmt.Sprintf("%s/%s@%s", owner, name, reference)
}
