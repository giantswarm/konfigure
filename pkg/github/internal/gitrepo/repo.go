package gitrepo

import (
	"context"

	"github.com/giantswarm/microerror"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

type Config struct {
	GitHubToken string
}

type Repo struct {
	gitHubToken string
}

func New(config Config) (*Repo, error) {
	r := &Repo{
		gitHubToken: config.GitHubToken,
	}

	return r, nil
}

func (r *Repo) ShallowCloneTag(ctx context.Context, url, tag string) (*Store, error) {
	return r.ShallowClone(ctx, url, plumbing.NewTagReferenceName(tag))
}

func (r *Repo) ShallowCloneBranch(ctx context.Context, url, branch string) (*Store, error) {
	return r.ShallowClone(ctx, url, plumbing.NewBranchReferenceName(branch))
}

func (r *Repo) ShallowClone(ctx context.Context, url string, ref plumbing.ReferenceName) (*Store, error) {
	var auth transport.AuthMethod
	if r.gitHubToken != "" {
		auth = &http.BasicAuth{
			Username: "can-be-anything-but-not-empty",
			Password: r.gitHubToken,
		}
	}

	fs := memfs.New()
	_, err := git.CloneContext(ctx, memory.NewStorage(), fs, &git.CloneOptions{
		Auth:          auth,
		URL:           url,
		ReferenceName: ref,
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	store := &Store{
		fs: fs,
	}

	return store, err
}
