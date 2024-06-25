package eywa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	endpoint   string
	httpClient *http.Client
	headers    map[string]string
}

type ClientOpts struct {
	HttpClient *http.Client
	Headers    map[string]string
}

// NewClient accepts a graphql endpoint and returns back a Client.
// It uses the http.DefaultClient as the underlying http client by default.
func NewClient(gqlEndpoint string, opt *ClientOpts) *Client {
	c := &Client{
		endpoint:   gqlEndpoint,
		httpClient: http.DefaultClient,
	}

	if opt != nil {
		if opt.HttpClient != nil {
			c.httpClient = opt.HttpClient
		}

		if opt.Headers != nil && len(opt.Headers) > 0 {
			c.headers = opt.Headers
		}
	}

	return c
}

func (c *Client) Do(q Queryable) (*bytes.Buffer, error) {
	reqObj := graphqlRequest{
		Query:     q.Query(),
		Variables: q.Variables(),
	}

	var reqBytes bytes.Buffer
	err := json.NewEncoder(&reqBytes).Encode(&reqObj)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.endpoint, &reqBytes)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	for key, value := range c.headers {
		req.Header.Add(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode > 299 && resp.StatusCode < 399:
		return nil, fmt.Errorf("redirected request with http status code: %d", resp.StatusCode)
	case resp.StatusCode > 399:
		return nil, fmt.Errorf("error response with http status code: %d", resp.StatusCode)
	}

	var respBytes bytes.Buffer
	_, err = io.Copy(&respBytes, resp.Body)
	return &respBytes, err
}
