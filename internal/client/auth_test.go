package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBackendGetRefreshesTokenAfterUnauthorized(t *testing.T) {
	oldGetValidToken := getValidToken
	oldForceRefreshToken := forceRefreshToken
	defer func() {
		getValidToken = oldGetValidToken
		forceRefreshToken = oldForceRefreshToken
	}()

	getCalls := 0
	refreshCalls := 0
	getValidToken = func() (string, error) {
		getCalls++
		return "stale-token", nil
	}
	forceRefreshToken = func() (string, error) {
		refreshCalls++
		return "fresh-token", nil
	}

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
		case 1:
			if got := r.Header.Get("Authorization"); got != "Bearer stale-token" {
				t.Fatalf("first request auth header = %q", got)
			}
			http.Error(w, `{"message":"invalid token"}`, http.StatusUnauthorized)
		case 2:
			if got := r.Header.Get("Authorization"); got != "Bearer fresh-token" {
				t.Fatalf("second request auth header = %q", got)
			}
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		default:
			t.Fatalf("unexpected request #%d", requests)
		}
	}))
	defer srv.Close()

	c := &Client{BackendURL: srv.URL}
	resp, err := c.BackendGet("/api/builds/demo")
	if err != nil {
		t.Fatalf("BackendGet returned error: %v", err)
	}
	if string(resp) != `{"status":"ok"}` {
		t.Fatalf("BackendGet body = %q", string(resp))
	}
	if getCalls != 1 {
		t.Fatalf("getCalls = %d, want 1", getCalls)
	}
	if refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", refreshCalls)
	}
}

func TestBackendGetDoesNotRetryStaticToken(t *testing.T) {
	oldGetValidToken := getValidToken
	oldForceRefreshToken := forceRefreshToken
	defer func() {
		getValidToken = oldGetValidToken
		forceRefreshToken = oldForceRefreshToken
	}()

	getValidToken = func() (string, error) {
		t.Fatal("unexpected dynamic token lookup")
		return "", nil
	}
	forceRefreshToken = func() (string, error) {
		t.Fatal("unexpected refresh token lookup")
		return "", nil
	}

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.Header.Get("Authorization"); got != "Bearer env-token" {
			t.Fatalf("auth header = %q", got)
		}
		http.Error(w, `{"message":"invalid token"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{BackendURL: srv.URL, Token: "env-token", StaticToken: true}
	_, err := c.BackendGet("/api/builds/demo")
	if err == nil {
		t.Fatal("expected BackendGet to fail")
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}
