package client

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type tasksRoundTripper func(*http.Request) (*http.Response, error)

func (fn tasksRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestListTasksUsesCatalogAndAuthorRoutes(t *testing.T) {
	originalHTTPClient := http.DefaultClient
	defer func() {
		http.DefaultClient = originalHTTPClient
	}()

	var called []string
	http.DefaultClient = &http.Client{Transport: tasksRoundTripper(func(r *http.Request) (*http.Response, error) {
		called = append(called, r.URL.String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("[]")),
		}, nil
	})}

	client := &Client{
		BackendURL:  "https://backend.test",
		Token:       "token",
		StaticToken: true,
	}

	if _, err := client.ListTasks(ListTasksOptions{Limit: 5}); err != nil {
		t.Fatalf("ListTasks(public) error = %v", err)
	}
	if _, err := client.ListTasks(ListTasksOptions{Mine: true, Limit: 7, Status: "draft"}); err != nil {
		t.Fatalf("ListTasks(mine) error = %v", err)
	}

	if len(called) != 2 {
		t.Fatalf("called = %#v, want 2 requests", called)
	}
	if !strings.Contains(called[0], "/api/tasks/catalog?limit=5") {
		t.Fatalf("public path = %q, want catalog route", called[0])
	}
	if !strings.Contains(called[1], "/api/tasks?limit=7&status=draft") {
		t.Fatalf("mine path = %q, want author route", called[1])
	}
}

func TestGetTaskUsesCatalogRoute(t *testing.T) {
	originalHTTPClient := http.DefaultClient
	defer func() {
		http.DefaultClient = originalHTTPClient
	}()

	var called string
	http.DefaultClient = &http.Client{Transport: tasksRoundTripper(func(r *http.Request) (*http.Response, error) {
		called = r.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"slug":"repair-the-ledger"}`)),
		}, nil
	})}

	client := &Client{
		BackendURL:  "https://backend.test",
		Token:       "token",
		StaticToken: true,
	}

	if _, err := client.GetTask("repair-the-ledger"); err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !strings.HasSuffix(called, "/api/tasks/catalog/repair-the-ledger") {
		t.Fatalf("called = %q, want catalog task route", called)
	}
}
