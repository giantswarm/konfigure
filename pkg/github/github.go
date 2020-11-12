package github

import (
	"context"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/config-controller/pkg/github/internal/graphql"
)

type Config struct {
	Token string
}

type GitHub struct {
	graphQLClient *graphql.Client
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

	g := &GitHub{
		graphQLClient: graphQLClient,
	}

	return g, nil
}

// Tags returns a list of tags for the given owner/name. Only tags containing
// filter string are returned. When filter is empty all tags are returned.
func (g *GitHub) Tags(ctx context.Context, owner, name, filter string) ([]string, error) {
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
