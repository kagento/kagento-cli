package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Credentials stored in ~/.kagento/credentials.json
type Credentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	ServerURL    string `json:"server_url"`
}

func credentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".kagento", "credentials.json"), nil
}

// Save writes credentials to ~/.kagento/credentials.json
func Save(creds *Credentials) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

// Load reads credentials from ~/.kagento/credentials.json
func Load() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return &creds, nil
}

// Delete removes the credentials file.
func Delete() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// GetValidToken returns a valid access token, refreshing if needed.
func GetValidToken() (string, error) {
	creds, err := Load()
	if err != nil {
		return "", err
	}

	// If token is still valid (with 30s buffer), return it.
	if time.Now().Unix() < creds.ExpiresAt-30 {
		return creds.AccessToken, nil
	}

	// Token expired — try to refresh via Supabase.
	if creds.RefreshToken == "" {
		return "", fmt.Errorf("token expired and no refresh token available")
	}

	if creds.ServerURL == "" {
		return "", fmt.Errorf("no server URL in credentials, please re-login")
	}

	tokenURL := creds.ServerURL + "/auth/v1/token?grant_type=refresh_token"
	reqBody, _ := json.Marshal(map[string]string{
		"refresh_token": creds.RefreshToken,
	})

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return "", fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", supabaseAnonKey(creds.ServerURL))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("parse refresh response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("refresh failed: %s — %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	newCreds := &Credentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Unix() + tokenResp.ExpiresIn,
		ServerURL:    creds.ServerURL,
	}
	if err := Save(newCreds); err != nil {
		return "", fmt.Errorf("save refreshed credentials: %w", err)
	}

	return newCreds.AccessToken, nil
}

// supabaseAnonKey returns the public anon key for the Supabase project.
// This is a public key (safe to embed) — it only grants anonymous access.
func supabaseAnonKey(supabaseURL string) string {
	// Default Kagento project anon key.
	return "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImJraG15YXRkd2ZheWRoY21na3RlIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NzQwODIwODMsImV4cCI6MjA4OTY1ODA4M30.DYAeIkk8Zb8gUBaSvByPKXp-xsYpL0uKfq7LOrasJks"
}

// ParseJWTClaims decodes the payload of a JWT without verification.
func ParseJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Base64url decode the payload (part 1).
	payload := parts[1]
	// Add padding if needed.
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	payload = strings.ReplaceAll(payload, "-", "+")
	payload = strings.ReplaceAll(payload, "_", "/")

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, fmt.Errorf("parse JWT claims: %w", err)
	}
	return claims, nil
}
