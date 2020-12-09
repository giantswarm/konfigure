package github

import (
	"context"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/config-controller/pkg/github"
	"github.com/giantswarm/config-controller/service/internal/github/cache"
)

type Config struct {
	GitHubToken string
}

type GitHub struct {
	client    *github.GitHub
	repoCache *cache.Repository
	tagCache  *cache.Tag
}

func New(c Config) (*GitHub, error) {
	client, err := github.New(github.Config{
		Token: c.GitHubToken,
	})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	gh := &GitHub{
		client:    client,
		repoCache: cache.NewRepository(),
		tagCache:  cache.NewTag(),
	}
	return gh, nil
}

func (gh *GitHub) GetLatestTag(ctx context.Context, owner, name, tagReference string) (string, error) {
	key := gh.tagCache.Key(owner, name, tagReference)
	tag, cached := gh.tagCache.Get(ctx, key)
	if cached {
		return tag, nil
	}

	tag, err := gh.client.GetLatestTag(ctx, owner, name, tagReference)
	if err != nil {
		return "", microerror.Mask(err)
	}

	gh.tagCache.Set(ctx, key, tag)
	return tag, nil
}

func (gh *GitHub) GetFilesByTag(ctx context.Context, owner, name, tag string) (github.Store, error) {
	key := gh.repoCache.Key(owner, name, tag)
	store, cached := gh.repoCache.Get(ctx, key)
	if cached {
		return store, nil
	}

	store, err := gh.client.GetFilesByTag(ctx, owner, name, tag)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	gh.repoCache.Set(ctx, key, store)
	return store, nil
}

func (gh *GitHub) GetFilesByBranch(ctx context.Context, owner, name, branch string) (github.Store, error) {
	key := gh.repoCache.Key(owner, name, branch)
	store, cached := gh.repoCache.Get(ctx, key)
	if cached {
		return store, nil
	}

	store, err := gh.client.GetFilesByBranch(ctx, owner, name, branch)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	gh.repoCache.Set(ctx, key, store)
	return store, nil
}
