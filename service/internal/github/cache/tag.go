package cache

import (
	"context"
	"fmt"

	gocache "github.com/patrickmn/go-cache"
)

type Tag struct {
	underlying *gocache.Cache
}

func NewTag() *Tag {
	return &Tag{
		underlying: gocache.New(expiration, expiration/2),
	}
}

func (t *Tag) Get(ctx context.Context, key string) (string, bool) {
	val, ok := t.underlying.Get(key)
	if ok {
		return val.(string), ok
	}

	return "", false
}

func (t *Tag) Set(ctx context.Context, key, value string) {
	t.underlying.SetDefault(key, value)
}

func (t *Tag) Key(owner, name, tag string) string {
	return fmt.Sprintf("%s/%s@%s", owner, name, tag)
}
