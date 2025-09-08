package models

import (
	"encoding/json"
	"time"
)

const (
	StatusPending = "PENDING"
	StatusQueued = "QUEUED"
	StatusRunning = "RUNNING"
	StatusCompleted = "COMPLETED"
	StatusFailed = "FAILED"
	StatusRetrying = "RETRYING"
)

type TaskConfig map[string]any

func (tc TaskConfig) Value() ([]byte, error) {
	return json.Marshal(tc)
}

func (tc *TaskConfig) Scan(value any) error {
	if value == nil {
		return nil
	}

	return json.Unmarshal(value.([]byte), tc)
}

type Task struct {
	ID               string         `json:"id" db:"id"`
	Type             string         `json:"type" db:"type"`
	Status           string         `json:"status" db:"status"`
	Priority         int	          `json:"priority" db:"priority"`
	UserID					 *int64					`json:"user_id,omitempty" db:"user_id"`
	ResourceID			 *string				`json:"resource_id,omitempty" db:"resource_id"`
	Config					 *TaskConfig 		`json:"config" db:"config"`
	CreatedAt        *time.Time     `json:"created_at" db:"created_at"`
	StartedAt        *time.Time     `json:"started_at" db:"started_at"`
	CompletedAt      *time.Time     `json:"completed_at" db:"completed_at"`
	RetryCount       int            `json:"retry_count" db:"retry_count"`
	MaxRetries       int            `json:"max_retries" db:"max_retry"`
	AssignedWorkerID *string        `json:"assigned_worker_id" db:"assigned_worker_id"`
	WorkedAssignedAt *time.Time     `json:"worker_assingned_at" db:"worker_assigned_at"`
}

func (t *Task) isCompleted() bool {
	return t.Status == StatusCompleted || t.Status == StatusFailed
}

func (t *Task) CanRetry() bool {
	return t.RetryCount < t.MaxRetries && t.Status == StatusFailed
}

func (t *Task) Duration() time.Duration {
	if t.StartedAt != nil && t.CompletedAt != nil {
		return t.CompletedAt.Sub(*t.StartedAt)
	}
	return 0
}
