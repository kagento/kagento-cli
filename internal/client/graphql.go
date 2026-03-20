package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Client sends GraphQL queries and mutations to the Kagento API.
type Client struct {
	URL         string // KAGENTO_API
	BackendURL  string // KAGENTO_BACKEND
	Registry    string // KAGENTO_REGISTRY
	Token       string // KAGENTO_TOKEN
	AdminSecret string // KAGENTO_ADMIN_SECRET
	UserID      string // KAGENTO_USER_ID
}

// NewClientFromEnv creates a Client populated from environment variables.
func NewClientFromEnv() *Client {
	return &Client{
		URL:         envOr("KAGENTO_API", "http://localhost:8080/v1/graphql"),
		BackendURL:  envOr("KAGENTO_BACKEND", "http://localhost:8081"),
		Registry:    envOr("KAGENTO_REGISTRY", "localhost:5000"),
		Token:       os.Getenv("KAGENTO_TOKEN"),
		AdminSecret: os.Getenv("KAGENTO_ADMIN_SECRET"),
		UserID:      os.Getenv("KAGENTO_USER_ID"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// graphqlRequest is the JSON body sent to the GraphQL endpoint.
type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// Query sends a GraphQL request and returns the parsed "data" object.
// It returns an error if the response contains GraphQL errors.
func (c *Client) Query(query string, variables map[string]interface{}) (map[string]interface{}, error) {
	body, err := json.Marshal(graphqlRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if errs, ok := result["errors"]; ok {
		errJSON, _ := json.MarshalIndent(errs, "", "  ")
		return nil, fmt.Errorf("GraphQL errors:\n%s", string(errJSON))
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response: %s", string(respBody))
	}
	return data, nil
}

// headers returns auth headers based on token or admin secret.
func (c *Client) headers() map[string]string {
	h := make(map[string]string)
	if c.Token != "" {
		h["Authorization"] = "Bearer " + c.Token
	} else if c.AdminSecret != "" {
		h["x-hasura-admin-secret"] = c.AdminSecret
		if c.UserID != "" {
			h["x-hasura-role"] = "user"
			h["x-hasura-user-id"] = c.UserID
		}
	}
	return h
}
