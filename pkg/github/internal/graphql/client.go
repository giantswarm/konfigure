package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/giantswarm/microerror"
)

type Config struct {
	Headers map[string]string
	URL     string
}

type Client struct {
	headers map[string]string
	url     string
}

func New(config Config) (*Client, error) {
	if config.URL == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.URL must not be empty", config)
	}

	c := &Client{
		headers: config.Headers,
		url:     config.URL,
	}

	return c, nil
}

func (c *Client) Do(ctx context.Context, req Request, v interface{}) error {
	var err error

	type Response struct {
		Data   interface{} `json:"data,omitempty"`
		Errors []struct {
			Message   string `json:"message,omitempty"`
			Locations []struct {
				Line   int `json:"line,omitempty"`
				Column int `json:"column,omitempty"`
			} `json:"locations,omitempty"`
		} `json:"errors,omitempty"`
	}

	requestData, err := json.Marshal(req)
	if err != nil {
		return microerror.Mask(err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(requestData))
	if err != nil {
		return microerror.Mask(err)
	}
	for k, v := range c.headers {
		httpReq.Header.Add(k, v)
	}

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return microerror.Mask(err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		bytes, _ := ioutil.ReadAll(httpResp.Body)
		return microerror.Maskf(executionFailedError, "expected status code = %d but got %d, response body = %#q", http.StatusOK, httpResp.StatusCode, bytes)
	}

	resp := Response{
		Data: v,
	}
	err = json.NewDecoder(httpResp.Body).Decode(&resp)
	if err != nil {
		return microerror.Mask(err)
	}

	if len(resp.Errors) > 0 {
		return microerror.Maskf(executionFailedError, "failed to execute GraphQL query with errors: %+v", resp.Errors)
	}

	return nil
}
