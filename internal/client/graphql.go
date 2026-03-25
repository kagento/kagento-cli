package client

import (
	"os"
)

// Client sends requests to the Kagento API.
type Client struct {
	BackendURL  string // KAGENTO_BACKEND
	Registry    string // KAGENTO_REGISTRY
	Token       string // KAGENTO_TOKEN
	AdminSecret string // KAGENTO_ADMIN_SECRET
	UserID      string // KAGENTO_USER_ID
}

// NewClientFromEnv creates a Client populated from environment variables.
func NewClientFromEnv() *Client {
	return &Client{
		BackendURL:  envOr("KAGENTO_BACKEND", "https://kagento.io"),
		Registry:    envOr("KAGENTO_REGISTRY", "registry.kagento.io"),
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

// headers returns auth headers based on token or admin secret.
func (c *Client) headers() map[string]string {
	h := make(map[string]string)
	if c.Token != "" {
		h["Authorization"] = "Bearer " + c.Token
	} else if c.AdminSecret != "" {
		h["Authorization"] = "Bearer " + c.AdminSecret
	}
	return h
}
