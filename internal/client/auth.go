package client

import (
	"fmt"
	"net/http"

	"github.com/kagento/kagento-cli/internal/auth"
)

var (
	getValidToken     = auth.GetValidToken
	forceRefreshToken = auth.ForceRefreshToken
)

func (c *Client) canRefreshAuth() bool {
	return !c.StaticToken && c.AdminSecret == ""
}

func (c *Client) setAuthHeader(req *http.Request, forceRefresh bool) error {
	if c.AdminSecret != "" {
		req.Header.Set("Authorization", "Bearer "+c.AdminSecret)
		return nil
	}

	if c.StaticToken {
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		return nil
	}

	var (
		token string
		err   error
	)
	if forceRefresh {
		token, err = forceRefreshToken()
	} else {
		token, err = getValidToken()
	}
	if err != nil {
		if c.Token != "" && !forceRefresh {
			req.Header.Set("Authorization", "Bearer "+c.Token)
			return nil
		}
		if forceRefresh {
			return err
		}
		return nil
	}

	c.Token = token
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

func (c *Client) doWithAuthRetry(buildRequest func(forceRefresh bool) (*http.Request, error)) (*http.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		forceRefresh := attempt == 1
		req, err := buildRequest(forceRefresh)
		if err != nil {
			return nil, err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 && c.canRefreshAuth() {
			c.Token = ""
			_ = resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after token refresh retry")
}
