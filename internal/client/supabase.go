package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	defaultSupabaseURL = "https://bkhmyatdwfaydhcmgkte.supabase.co"
	supabaseAnonKey    = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImJraG15YXRkd2ZheWRoY21na3RlIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NzQwODIwODMsImV4cCI6MjA4OTY1ODA4M30.DYAeIkk8Zb8gUBaSvByPKXp-xsYpL0uKfq7LOrasJks"
)

// SupabaseGet sends a GET request to the Supabase PostgREST API.
// The path should start with /rest/v1/ (e.g., "/rest/v1/sessions?id=eq.xxx").
func (c *Client) SupabaseGet(path string) ([]byte, error) {
	supabaseURL := envOr("SUPABASE_URL", defaultSupabaseURL)
	url := strings.TrimRight(supabaseURL, "/") + path

	resp, err := c.doWithAuthRetry(func(forceRefresh bool) (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("apikey", supabaseAnonKey)
		if err := c.setAuthHeader(req, forceRefresh); err != nil {
			return nil, err
		}
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// SupabasePatch sends a PATCH request to the Supabase PostgREST API.
func (c *Client) SupabasePatch(path string, body map[string]interface{}) ([]byte, error) {
	supabaseURL := envOr("SUPABASE_URL", defaultSupabaseURL)
	url := strings.TrimRight(supabaseURL, "/") + path

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	resp, err := c.doWithAuthRetry(func(forceRefresh bool) (*http.Request, error) {
		req, err := http.NewRequest("PATCH", url, strings.NewReader(string(data)))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("apikey", supabaseAnonKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Prefer", "return=minimal")
		if err := c.setAuthHeader(req, forceRefresh); err != nil {
			return nil, err
		}
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// SupabasePost sends a POST request to the Supabase PostgREST API.
func (c *Client) SupabasePost(path string, body map[string]interface{}) ([]byte, error) {
	supabaseURL := envOr("SUPABASE_URL", defaultSupabaseURL)
	url := strings.TrimRight(supabaseURL, "/") + path

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	resp, err := c.doWithAuthRetry(func(forceRefresh bool) (*http.Request, error) {
		req, err := http.NewRequest("POST", url, strings.NewReader(string(data)))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("apikey", supabaseAnonKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Prefer", "return=representation")
		if err := c.setAuthHeader(req, forceRefresh); err != nil {
			return nil, err
		}
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
