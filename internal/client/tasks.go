package client

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type TaskRecord struct {
	ID               string          `json:"id"`
	Slug             string          `json:"slug"`
	Title            string          `json:"title"`
	ShortDesc        string          `json:"short_desc"`
	Description      string          `json:"description,omitempty"`
	Difficulty       string          `json:"difficulty,omitempty"`
	Status           string          `json:"status,omitempty"`
	Size             string          `json:"size,omitempty"`
	TimeLimitSec     int             `json:"time_limit_sec,omitempty"`
	EnvironmentType  string          `json:"environment_type,omitempty"`
	TaskInstructions string          `json:"task_instructions,omitempty"`
	ScoringType      string          `json:"scoring_type,omitempty"`
	ScoringConfig    json.RawMessage `json:"scoring_config,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
	ReviewFeedback   string          `json:"review_feedback,omitempty"`
	CreatedAt        string          `json:"created_at,omitempty"`
}

type ListTasksOptions struct {
	Mine   bool
	Status string
	Limit  int
}

func (c *Client) ListTasks(opts ListTasksOptions) ([]TaskRecord, error) {
	query := url.Values{}
	if opts.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Status != "" {
		query.Set("status", opts.Status)
	}

	path := "/api/catalog/tasks"
	if opts.Mine {
		path = "/api/tasks"
	}

	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	resp, err := c.BackendGet(path)
	if err != nil {
		return nil, err
	}
	var rows []TaskRecord
	if err := json.Unmarshal(resp, &rows); err != nil {
		return nil, fmt.Errorf("parse task list: %w", err)
	}
	return rows, nil
}

func (c *Client) GetTask(slug string) (*TaskRecord, error) {
	resp, err := c.BackendGet("/api/catalog/tasks/" + url.PathEscape(slug))
	if err != nil {
		return nil, err
	}
	var task TaskRecord
	if err := json.Unmarshal(resp, &task); err != nil {
		return nil, fmt.Errorf("parse task: %w", err)
	}
	return &task, nil
}

func (c *Client) GetOwnTask(slug string) (*TaskRecord, error) {
	resp, err := c.BackendGet("/api/tasks/" + url.PathEscape(slug))
	if err != nil {
		return nil, err
	}
	var task TaskRecord
	if err := json.Unmarshal(resp, &task); err != nil {
		return nil, fmt.Errorf("parse task: %w", err)
	}
	return &task, nil
}

func (c *Client) UpdateTask(slug string, params map[string]any) (*TaskRecord, error) {
	resp, err := c.backendPatch("/api/tasks/"+url.PathEscape(slug), params)
	if err != nil {
		return nil, err
	}
	var task TaskRecord
	if err := json.Unmarshal(resp, &task); err != nil {
		return nil, fmt.Errorf("parse updated task: %w", err)
	}
	return &task, nil
}
