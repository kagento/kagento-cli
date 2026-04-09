package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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
	columns := []string{
		"id", "slug", "title", "short_desc", "description", "difficulty",
		"status", "size", "time_limit_sec", "environment_type",
		"task_instructions", "scoring_type", "scoring_config", "tags",
		"review_feedback", "created_at",
	}
	query.Set("select", strings.Join(columns, ","))
	query.Set("order", "created_at.desc")
	if opts.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Status != "" {
		query.Set("status", "eq."+opts.Status)
	} else if !opts.Mine {
		query.Set("status", "eq.published")
	}
	if opts.Mine {
		userID, err := c.CurrentUserID()
		if err != nil {
			return nil, err
		}
		if userID == "" {
			return nil, fmt.Errorf("not logged in")
		}
		query.Set("author_id", "eq."+userID)
	}

	resp, err := c.SupabaseGet("/rest/v1/tasks?" + query.Encode())
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
	query := url.Values{}
	query.Set("slug", "eq."+slug)
	query.Set("select", "id,slug,title,short_desc,description,difficulty,status,size,time_limit_sec,environment_type,task_instructions,scoring_type,scoring_config,tags,review_feedback,created_at")
	query.Set("limit", "1")

	resp, err := c.SupabaseGet("/rest/v1/tasks?" + query.Encode())
	if err != nil {
		return nil, err
	}
	var rows []TaskRecord
	if err := json.Unmarshal(resp, &rows); err != nil {
		return nil, fmt.Errorf("parse task: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("task not found")
	}
	return &rows[0], nil
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
