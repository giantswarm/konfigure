package github

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/config-controller/pkg/github/internal/gitrepo"
	"github.com/giantswarm/config-controller/pkg/github/internal/graphql"
)

type Config struct {
	Token string
}

type GitHub struct {
	graphQLClient *graphql.Client
	repo          *gitrepo.Repo
}

func New(config Config) (*GitHub, error) {
	if config.Token == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Token must not be empty", config)
	}

	var err error

	var graphQLClient *graphql.Client
	{
		c := graphql.Config{
			Headers: map[string]string{
				"Authorization": "bearer " + config.Token,
			},
			URL: "https://api.github.com/graphql",
		}
		graphQLClient, err = graphql.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var repo *gitrepo.Repo
	{
		c := gitrepo.Config{
			GitHubToken: config.Token,
		}

		repo, err = gitrepo.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	g := &GitHub{
		graphQLClient: graphQLClient,
		repo:          repo,
	}

	return g, nil
}

func (g *GitHub) GetLatestTag(ctx context.Context, owner, name, major string) (string, error) {
	tags, err := g.getTags(ctx, owner, name, major)
	if err != nil {
		return "", microerror.Mask(err)
	}

	latest := getLatestTag(tags)

	if latest == "" {
		return "", microerror.Maskf(executionFailedError, "did not find tag for `%s/%s` for major %#q", owner, name, major)
	}

	return latest, nil
}

func (g *GitHub) GetFiles(ctx context.Context, owner, name, tag string) (Store, error) {
	url := "https://github.com/" + owner + "/" + name + ".git"
	store, err := g.repo.ShallowClone(ctx, url, tag)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return store, nil
}

// getTags returns a list of tags for the given owner/name. Only tags containing
// filter string are returned. When filter is empty all tags are returned.
func (g *GitHub) getTags(ctx context.Context, owner, name, filter string) ([]string, error) {
	const query = `
		query($owner:String!, $name:String!, $filter:String!, $after:String) {
		  repository(name: $name, owner: $owner) {
		    refs(first: 100, refPrefix: "refs/tags/", after: $after, query: $filter, direction: ASC) {
		      edges {
		        cursor
		        node {
		          name
		        }
		      }
		    }
		  }
		}
	`

	type Variables struct {
		After  string `json:"after"`
		Filter string `json:"filter"`
		Name   string `json:"name"`
		Owner  string `json:"owner"`
	}

	type Data struct {
		Repository struct {
			Refs struct {
				Edges []struct {
					Cursor string `json:"cursor"`
					Node   struct {
						Name string `json:"name"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"refs"`
		} `json:"repository"`
	}

	var tags []string
	var after string
	for {
		req := graphql.Request{
			Query: query,
			Variables: Variables{
				After:  after,
				Filter: filter,
				Name:   name,
				Owner:  owner,
			},
		}

		var d Data
		err := g.graphQLClient.Do(ctx, req, &d)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		if len(d.Repository.Refs.Edges) == 0 {
			break
		}

		for _, e := range d.Repository.Refs.Edges {
			// Direction is specified as ASC in the query so the
			// last cursor will be in the last element in the loop.
			after = e.Cursor
			tags = append(tags, e.Node.Name)
		}
	}

	return tags, nil
}

func getLatestTag(tags []string) string {
	re := regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

	type MajorMinorPatch [3]int

	var versions []MajorMinorPatch
	for _, t := range tags {
		subs := re.FindStringSubmatch(t)
		if len(subs) == 4 {
			major, err := strconv.Atoi(subs[1])
			if err != nil {
				panic(microerror.Pretty(microerror.Mask(err), true))
			}
			minor, err := strconv.Atoi(subs[2])
			if err != nil {
				panic(microerror.Pretty(microerror.Mask(err), true))
			}
			patch, err := strconv.Atoi(subs[3])
			if err != nil {
				panic(microerror.Pretty(microerror.Mask(err), true))
			}

			versions = append(versions, MajorMinorPatch{major, minor, patch})
		}
	}

	lessFunc := func(i, j int) bool {
		x := versions[i]
		y := versions[j]
		switch {
		case x[0] < y[0]:
			return true
		case x[0] > y[0]:
			return false
		case x[1] < y[1]:
			return true
		case x[1] > y[1]:
			return false
		case x[2] < y[2]:
			return true
		case x[2] > y[2]:
			return false
		}
		return false
	}

	sort.Slice(versions, lessFunc)

	if len(versions) == 0 {
		return ""
	}

	v := versions[len(versions)-1]
	return fmt.Sprintf("v%d.%d.%d", v[0], v[1], v[2])
}
