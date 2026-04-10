package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ResolveSessionID accepts either a UUID or a two-word ssh_session_name and returns the session UUID.
func (c *Client) ResolveSessionID(idOrName string) (string, error) {
	if uuidRe.MatchString(idOrName) {
		return idOrName, nil
	}
	// Treat as ssh_session_name.
	resp, err := c.SupabaseGet(
		"/rest/v1/sessions?ssh_session_name=eq." + idOrName +
			"&select=id" +
			"&status=in.(pending,ready,running,finishing)" +
			"&order=created_at.desc&limit=1",
	)
	if err != nil {
		return "", err
	}
	var rows []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &rows); err != nil {
		return "", fmt.Errorf("parse session lookup: %w", err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no active session found with name %q", idOrName)
	}
	return rows[0].ID, nil
}

// BuildStatus represents the status of a task build.
type BuildStatus struct {
	ID              string `json:"id"`
	Slug            string `json:"slug"`
	Status          string `json:"status"`
	UserImageDigest string `json:"user_image_digest,omitempty"`
	TestImageDigest string `json:"test_image_digest,omitempty"`
	UserBaseImage   string `json:"user_base_image,omitempty"`
	TestBaseImage   string `json:"test_base_image,omitempty"`
	Signed          bool   `json:"signed"`
	Progress        string `json:"progress,omitempty"`
	StepTimings     any    `json:"step_timings,omitempty"`
	Error           string `json:"error,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	CompletedAt     string `json:"completed_at,omitempty"`
}

type ListBuildsOptions struct {
	Slug   string
	Status string
	Limit  int
}

type RegistryCredentials struct {
	ExpiresIn int    `json:"expires_in"`
	Password  string `json:"password"`
	Registry  string `json:"registry"`
	Username  string `json:"username"`
}

// StartBuild uploads a source tar and triggers a server-side build.
func (c *Client) StartBuild(slug, sourcePath string) (string, error) {
	resp, err := c.doBuildUpload(slug, sourcePath)
	if err != nil {
		return "", err
	}
	var result struct {
		BuildID string `json:"build_id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parse start build response: %w", err)
	}
	return result.BuildID, nil
}

// GetBuild returns the status of a specific build.
func (c *Client) GetBuild(buildID string) (*BuildStatus, error) {
	resp, err := c.BackendGet("/api/builds/" + buildID)
	if err != nil {
		return nil, err
	}
	var result BuildStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse build status: %w", err)
	}
	return &result, nil
}

// ListBuilds returns builds for a given slug.
func (c *Client) ListBuilds(slug string) ([]BuildStatus, error) {
	return c.ListBuildsWithOptions(ListBuildsOptions{Slug: slug})
}

func (c *Client) ListBuildsWithOptions(opts ListBuildsOptions) ([]BuildStatus, error) {
	query := url.Values{}
	if opts.Slug != "" {
		query.Set("slug", opts.Slug)
	}
	if opts.Status != "" {
		query.Set("status", opts.Status)
	}
	if opts.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	path := "/api/builds"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	resp, err := c.BackendGet(path)
	if err != nil {
		return nil, err
	}
	var result []BuildStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse builds list: %w", err)
	}
	return result, nil
}

func (c *Client) RetryBuild(buildID string) (*BuildStatus, error) {
	resp, err := c.backendPost("/api/builds/"+buildID+"/retry", nil)
	if err != nil {
		return nil, err
	}
	var result BuildStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse retry response: %w", err)
	}
	return &result, nil
}

func (c *Client) CancelBuild(buildID string) (map[string]any, error) {
	resp, err := c.backendPost("/api/builds/"+buildID+"/cancel", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse cancel response: %w", err)
	}
	return result, nil
}

// PublishBuild promotes a completed build to a published task.
func (c *Client) PublishBuild(buildID string, params map[string]interface{}) (map[string]interface{}, error) {
	resp, err := c.backendPost("/api/builds/"+buildID+"/publish", params)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse publish response: %w", err)
	}
	return result, nil
}

// PublishK8sTask publishes a vcluster task directly (no Docker build).
func (c *Client) PublishK8sTask(params map[string]interface{}) (map[string]interface{}, error) {
	resp, err := c.backendPost("/api/tasks/publish-k8s", params)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse publish response: %w", err)
	}
	return result, nil
}

// StartSession creates a new session for a task via the backend API.
func (c *Client) StartSession(taskSlug string) (map[string]interface{}, error) {
	resp, err := c.BackendPost("/api/sessions/start", map[string]string{
		"task_slug": taskSlug,
	})
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse start session response: %w", err)
	}
	return result, nil
}

// GetSessionStatus fetches session status via Supabase PostgREST.
func (c *Client) GetSessionStatus(sessionID string) (map[string]interface{}, error) {
	resp, err := c.SupabaseGet(
		"/rest/v1/sessions?id=eq." + sessionID +
			"&select=id,status,started_at,finished_at,task:tasks!sessions_task_id_fkey(title)" +
			"&limit=1",
	)
	if err != nil {
		return nil, err
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(resp, &rows); err != nil {
		return nil, fmt.Errorf("parse session status: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("session not found")
	}
	row := rows[0]
	// Flatten task.title to task_title for CLI convenience.
	if task, ok := row["task"].(map[string]interface{}); ok {
		row["task_title"] = task["title"]
	}
	return row, nil
}

// FinishSession triggers the finish flow via the backend API.
// The backend sets status to "finishing" and the event handler runs tests.
func (c *Client) FinishSession(sessionID string) error {
	_, err := c.BackendPost("/api/sessions/"+sessionID+"/finish", nil)
	return err
}

// GetSessionSubmission fetches the latest submission for a session via Supabase PostgREST.
func (c *Client) GetSessionSubmission(sessionID string) (map[string]interface{}, error) {
	resp, err := c.SupabaseGet(
		"/rest/v1/submissions?session_id=eq." + sessionID +
			"&select=id,score,duration_sec,details,created_at" +
			"&order=created_at.desc&limit=1",
	)
	if err != nil {
		return nil, err
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(resp, &rows); err != nil {
		return nil, fmt.Errorf("parse submission: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no submission found")
	}
	return rows[0], nil
}

// CreateCheck creates a new check for a session via the backend API and returns the check ID.
func (c *Client) CreateCheck(sessionID string) (string, error) {
	resp, err := c.BackendPost("/api/sessions/"+sessionID+"/check", nil)
	if err != nil {
		return "", err
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parse check: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("failed to create check")
	}
	return result.ID, nil
}

// GetCheckStatus fetches check status via Supabase PostgREST.
func (c *Client) GetCheckStatus(checkID string) (map[string]interface{}, error) {
	resp, err := c.SupabaseGet(
		"/rest/v1/checks?id=eq." + checkID +
			"&select=id,status,score,details,completed_at" +
			"&limit=1",
	)
	if err != nil {
		return nil, err
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(resp, &rows); err != nil {
		return nil, fmt.Errorf("parse check status: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("check not found")
	}
	return rows[0], nil
}

// UnpublishTask sets a task back to draft via Supabase PostgREST.
func (c *Client) UnpublishTask(slug string) error {
	_, err := c.SupabasePatch(
		"/rest/v1/tasks?slug=eq."+slug+"&status=eq.published",
		map[string]interface{}{"status": "draft"},
	)
	return err
}

// ArchiveTask archives a task via Supabase PostgREST.
func (c *Client) ArchiveTask(slug string) error {
	_, err := c.SupabasePatch(
		"/rest/v1/tasks?slug=eq."+slug,
		map[string]interface{}{"status": "archived"},
	)
	return err
}

// backendPost sends a POST request to the backend API.
func (c *Client) backendPost(path string, body interface{}) ([]byte, error) {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
	}

	resp, err := c.doWithAuthRetry(func(forceRefresh bool) (*http.Request, error) {
		req, err := http.NewRequest("POST", c.BackendURL+path, bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
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

func (c *Client) backendPatch(path string, body interface{}) ([]byte, error) {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
	}

	resp, err := c.doWithAuthRetry(func(forceRefresh bool) (*http.Request, error) {
		req, err := http.NewRequest("PATCH", c.BackendURL+path, bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
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

// GetKubeconfig downloads the kubeconfig for a session via Supabase.
func (c *Client) GetKubeconfig(sessionID string) ([]byte, error) {
	resp, err := c.SupabaseGet(
		"/rest/v1/sessions?id=eq." + sessionID +
			"&select=kubeconfig" +
			"&limit=1",
	)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Kubeconfig *string `json:"kubeconfig"`
	}
	if err := json.Unmarshal(resp, &rows); err != nil {
		return nil, fmt.Errorf("parse kubeconfig response: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("session not found")
	}
	if rows[0].Kubeconfig == nil || *rows[0].Kubeconfig == "" {
		return nil, fmt.Errorf("kubeconfig not ready yet")
	}
	kc := *rows[0].Kubeconfig
	// Normalize escaped newlines.
	if strings.Contains(kc, "\\n") {
		kc = strings.ReplaceAll(kc, "\\n", "\n")
	}
	return []byte(kc), nil
}

func (c *Client) GetSessionRegistryCredentials(sessionID string) (*RegistryCredentials, error) {
	resp, err := c.BackendPost("/api/registry/token", map[string]string{
		"scope":      "session",
		"session_id": sessionID,
	})
	if err != nil {
		return nil, err
	}
	var result RegistryCredentials
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse registry credentials: %w", err)
	}
	if strings.TrimSpace(result.Registry) == "" || strings.TrimSpace(result.Username) == "" || strings.TrimSpace(result.Password) == "" {
		return nil, fmt.Errorf("registry credentials response was incomplete")
	}
	return &result, nil
}

// BackendGet sends a GET request to the backend API.
func (c *Client) BackendGet(path string) ([]byte, error) {
	resp, err := c.doWithAuthRetry(func(forceRefresh bool) (*http.Request, error) {
		req, err := http.NewRequest("GET", c.BackendURL+path, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
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

// BackendPost sends a POST request to the backend API.
func (c *Client) BackendPost(path string, body interface{}) ([]byte, error) {
	resp, err := c.doWithAuthRetry(func(forceRefresh bool) (*http.Request, error) {
		var reqBody io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshal body: %w", err)
			}
			reqBody = bytes.NewReader(data)
		}
		req, err := http.NewRequest("POST", c.BackendURL+path, reqBody)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
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

func (c *Client) doBuildUpload(slug, sourcePath string) ([]byte, error) {
	resp, err := c.doWithAuthRetry(func(forceRefresh bool) (*http.Request, error) {
		file, err := os.Open(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("open source archive: %w", err)
		}

		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		go func() {
			defer func() {
				_ = file.Close()
			}()

			if err := writer.WriteField("slug", slug); err != nil {
				_ = pw.CloseWithError(err)
				return
			}

			part, err := writer.CreateFormFile("source", filepath.Base(sourcePath))
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if _, err := io.Copy(part, file); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if err := writer.Close(); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_ = pw.Close()
		}()

		req, err := http.NewRequest("POST", c.BackendURL+"/api/builds/start", pr)
		if err != nil {
			_ = file.Close()
			_ = pr.Close()
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if err := c.setAuthHeader(req, forceRefresh); err != nil {
			_ = pr.Close()
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
