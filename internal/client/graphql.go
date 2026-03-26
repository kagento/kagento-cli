package client

import (
	"os"
)

// Client sends requests to the Kagento API.
type Client struct {
	BackendURL  string // KAGENTO_BACKEND
	Registry    string // KAGENTO_REGISTRY
	Token       string // KAGENTO_TOKEN
	StaticToken bool   // token came from KAGENTO_TOKEN and should not be refreshed
	AdminSecret string // KAGENTO_ADMIN_SECRET
	UserID      string // KAGENTO_USER_ID
}

// NewClientFromEnv creates a Client populated from environment variables.
func NewClientFromEnv() *Client {
	token := os.Getenv("KAGENTO_TOKEN")
	return &Client{
		BackendURL:  envOr("KAGENTO_BACKEND", "https://kagento.io"),
		Registry:    envOr("KAGENTO_REGISTRY", "registry.kagento.io"),
		Token:       token,
		StaticToken: token != "",
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
