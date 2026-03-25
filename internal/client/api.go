package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BuildStatus represents the status of a task build.
type BuildStatus struct {
	ID              string `json:"id"`
	Slug            string `json:"slug"`
	Status          string `json:"status"`
	UserImageDigest string `json:"user_image_digest,omitempty"`
	TestImageDigest string `json:"test_image_digest,omitempty"`
	Signed          bool   `json:"signed"`
	Error           string `json:"error,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	CompletedAt     string `json:"completed_at,omitempty"`
}

// PresignResponse holds the presigned upload URL and source key.
type PresignResponse struct {
	UploadURL string `json:"upload_url"`
	SourceKey string `json:"source_key"`
}

// PresignBuildUpload requests a presigned S3 URL for uploading a task source tar.
func (c *Client) PresignBuildUpload() (*PresignResponse, error) {
	resp, err := c.backendPost("/api/builds/presign", nil)
	if err != nil {
		return nil, err
	}
	var result PresignResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse presign response: %w", err)
	}
	return &result, nil
}

// StartBuild triggers a server-side build after uploading the source tar.
func (c *Client) StartBuild(slug, sourceKey string) (string, error) {
	body := map[string]string{
		"slug":       slug,
		"source_key": sourceKey,
	}
	resp, err := c.backendPost("/api/builds/start", body)
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
	resp, err := c.BackendGet("/api/builds?slug=" + slug)
	if err != nil {
		return nil, err
	}
	var result []BuildStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse builds list: %w", err)
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

// UploadToPresignedURL uploads data to a presigned S3 URL.
func UploadToPresignedURL(url string, data io.Reader, contentLength int64) error {
	req, err := http.NewRequest("PUT", url, data)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.ContentLength = contentLength
	req.Header.Set("Content-Type", "application/gzip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
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

// FinishSession triggers the finish flow for a session.
func (c *Client) FinishSession(sessionID string) error {
	_, err := c.backendPost("/api/sessions/"+sessionID+"/finish", nil)
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

// CreateCheck creates a new check for a session and returns the check ID.
func (c *Client) CreateCheck(sessionID string) (string, error) {
	resp, err := c.backendPost("/api/sessions/"+sessionID+"/check", nil)
	if err != nil {
		return "", err
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parse check: %w", err)
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
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest("POST", c.BackendURL+path, reqBody)
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

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetKubeconfig downloads the kubeconfig for a session.
func (c *Client) GetKubeconfig(sessionID string) ([]byte, error) {
	return c.BackendGet("/api/sessions/" + sessionID + "/kubeconfig")
}

// BackendGet sends a GET request to the backend API.
func (c *Client) BackendGet(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.BackendURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
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

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
