package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/cakra17/tq/internal/models"
	_ "github.com/lib/pq"
)

type TaskRepo struct {
	db *sql.DB
	lg *slog.Logger
}

func NewTaskRepo(db *sql.DB, lg *slog.Logger) TaskRepo {
	return TaskRepo{ db: db, lg: lg }
}

func (r *TaskRepo) AddTask(ctx context.Context,t models.Task) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.lg.Error("Database Error", "Failed to begin transaction %s", err.Error())
		return err
	}
	defer func () {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.lg.Error("Database Error", "Failed to rollback transaction", rbErr.Error())
				return
			}
		}
	}()

	query := `
		INSERT INTO tasks (
			id, type, status, priority, config, created_at, 
			started_at, completed_at, retry_count, max_retries,
			assigned_worker_id, worker_assigned_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, 
			$8, $9, $10, $11, $12
		)
	`

	_, err = r.db.ExecContext(ctx, query, 
		t.ID, t.Type, t.Status, t.Priority, t.Config, t.CreatedAt, 
		t.StartedAt, t.CompletedAt, t.RetryCount, t.MaxRetries,
		t.AssignedWorkerID, t.WorkedAssignedAt,
	)
	if err != nil {
		r.lg.Error("Database Error", "Failed to insert task", err.Error())
		return err
	}

	if err := tx.Commit(); err != nil {
		r.lg.Error("Database Error", "Failed to commit task", err.Error())
		return err
	}

	return nil
}

func (r *TaskRepo) GetTask(ctx context.Context, id string) (*models.Task, error) {
	if id == "" {
		return nil, fmt.Errorf("id is empty")
	}

	var t models.Task
	query := `
	SELECT 
		id, type, status, priority, config,
		created_at, started_at, completed_at, 
		retry_count, max_retries,
		assigned_worker_id, worker_assigned_at
	FROM tasks WHERE id = $1`
	
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&t.ID, &t.Type, &t.Status, &t.Priority, &t.Config,
		&t.CreatedAt, &t.StartedAt, &t.CompletedAt, 
		&t.RetryCount, &t.MaxRetries,
		&t.AssignedWorkerID, &t.WorkedAssignedAt,
	)

	if err == sql.ErrNoRows {
		r.lg.ErrorContext(ctx, "Database Error", "Failed to get task", err.Error())
		return nil, fmt.Errorf("Task not found")
	}

	if err != nil {
		r.lg.ErrorContext(ctx, "Database Error", "Failed to get task", err.Error())
		return nil, fmt.Errorf("Failed to get task: %v", err)
	}

	return &t, nil
}

func (r *TaskRepo) UpdateTaskStart(
	ctx context.Context, taskID, workerID, status string,
	retryCount int, startedAt, workedAssignAt *time.Time,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.lg.Error("Database Error", "Failed to begin transaction %s", err.Error())
		return err
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); err != nil {
				r.lg.Error("Database Error", "Failed to rollback transaction", rbErr.Error())
				return
			}
		}
	}()

	query := `
		UPDATE tasks
		SET status = $1, assigned_worker_id = $2, worker_assigned_at = $3, started_at = $4, retry_count = $5
		WHERE id = $6
	`
	_, err = r.db.ExecContext(ctx, query, status, workerID, workedAssignAt, startedAt, retryCount, taskID)
	if err != nil {
		r.lg.Error("Database Error", "Failed to update task", err.Error())
		return err
	}

	if err := tx.Commit(); err != nil {
		r.lg.Error("Database Error", "Failed to commit task", err.Error())
		return err
	}
	return nil
}

func (r *TaskRepo) UpdateTaskCompletion(
	ctx context.Context, taskID, status string, 
	completedAt *time.Time,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.lg.Error("Database Error", "Failed to begin transaction %s", err.Error())
		return err
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.lg.Error("Database Error", "Failed to rollback transaction", rbErr.Error())
				return 
			}
		}
	}()

	query := `
		UPDATE tasks
		SET status = $1, completed_at = $2
		WHERE id = $3
	`
	_, err = r.db.ExecContext(ctx, query, status, completedAt, taskID)
	if err != nil {
		r.lg.Error("Database Error", "Failed to update task completion", err.Error())
		return err
	}

	if err := tx.Commit(); err != nil {
		r.lg.Error("Database Error", "Failed to commit task", err.Error())
		return err
	}

	return nil
}

func (r *TaskRepo) UpdateTaskRetry(ctx context.Context, taskID string, retryCount int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.lg.Error("Database Error", "Failed to begin transaction %s", err.Error())
		return err
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.lg.Error("Database Error", "Failed to rollback transaction", rbErr.Error())
				return
			}
		}
	}()

	status := models.STATUSRETRIED

	query := `
		UPDATE tasks
		SET retry_count = $1, status = $2
		WHERE id = $3, 
	`
	_, err = r.db.ExecContext(ctx, query, retryCount, status, taskID)
	if err != nil {
		r.lg.Error("Database Error", "Failed to update task retry", err.Error())
		return err
	}

	if err := tx.Commit(); err != nil {
		r.lg.Error("Database Error", "Failed to commit task", err.Error())
		return err
	}

	return nil
}

func (r *TaskRepo) DeleteTask(ctx context.Context, taskID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func () {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.lg.Error("Database Error", "Failed to rollback transaction", err.Error())
				return
			}
		}
	}()

	query := `
		DELETE FROM tasks WHERE id = $1
	`
	_, err = r.db.ExecContext(ctx, query, taskID)
	if err != nil {
		r.lg.Error("Database Error", "Failed to delete task", err.Error())
		return err
	}

	if err := tx.Commit(); err != nil {
		r.lg.Error("Database Error", "Failed to commit task", err.Error())
		return err
	}

	return nil
}