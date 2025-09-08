package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/cakra17/tq/internal/config"
	"github.com/cakra17/tq/internal/models"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type Repo struct {
	db *sql.DB
	rd *redis.Client
}

func NewRepo(db *sql.DB, rd *redis.Client) Repo {
	return Repo{ 
		db: db,
		rd: rd,
	}
}

func (r *Repo) AddTask(ctx context.Context,t models.Task) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Failed to begin transaction %v", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO tasks (
			id, type, status, priority, user_id, resource_id, config,
			created_at, started_at, completed_at, retry_count, max_retries,
			assigned_worker_id, worked_assigned_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, 
			$8, $9, $10, $11, $12, $13, $14
		)
	`

	_, err = r.db.ExecContext(ctx, query, 
		t.ID, t.Type, t.Status, t.Priority, t.UserID, t.ResourceID, t.Config,
		t.CreatedAt, t.StartedAt, t.CompletedAt, t.RetryCount, t.MaxRetries,
		t.AssignedWorkerID, t.WorkedAssignedAt,
	)
	if err != nil {
		return fmt.Errorf("Failed to insert task: %v", err)
	}

	// using prior queue for redis
	score := float64(t.Priority * 1000000) + float64(t.CreatedAt.Unix())
	err = r.rd.ZAdd(ctx, "task_queue", redis.Z{
		Score: score,
		Member: t.ID,
	}).Err()
	if err != nil {
		return fmt.Errorf("Failed to add task to queue: %v", err)
	}

	return tx.Commit()
}

func (r *Repo) GetTask(ctx context.Context, id string) (*models.Task, error) {
	var t models.Task
	query := `
	SELECT 
		id, type, status, priority, user_id, resource_id, config,
		created_at, started_at, completed_at, retry_count, max_retries,
		assigned_worker_id, worker_assigned_at
	FROM tasks WHERE id = $1`
	
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&t.ID, &t.Type, &t.Status, &t.Priority, &t.UserID, &t.ResourceID, &t.Config,
		&t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.RetryCount, &t.MaxRetries,
		&t.AssignedWorkerID, &t.WorkedAssignedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("Task not found\n")
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to get task: %v", err)
	}

	return &t, nil
}

func (r *Repo) PullNextTask(ctx context.Context, workerID string) (*models.Task, error) {
	// Pop highest priority task (ZPOPMAX)
	result, err := r.rd.ZPopMax(ctx, "task_queue", 1).Result()
	if err == redis.Nil {
		return nil, nil // no tasks available
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to pop task from queue: %v", err)
	}

	if len(result) == 0 {
		return nil, nil
	}

	taskID := result[0].Member.(string)
	task, err := r.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task %s not found in database: %v", taskID, err)
	}

	now := time.Now()
	task.Status = models.StatusRunning
	task.StartedAt = &now
	task.AssignedWorkerID = &workerID
	task.WorkedAssignedAt = &now

	err = r.UpdateTaskStatus(ctx, taskID, task.Status, *task.AssignedWorkerID, task.StartedAt, task.WorkedAssignedAt)
	if err != nil {
		r.rd.ZAdd(ctx, "task_queue", redis.Z{
			Score: result[0].Score,
			Member: taskID,
		})
		return nil, fmt.Errorf("Failed to update task status: %v", err)
	}
	return task, nil
}

func (r *Repo) UpdateTaskStatus(
	ctx context.Context, taskID, status, workerID string, 
	startedAt, workedAssignAt *time.Time,
) error {
	query := `
		UPDATE tasks
		SET status = $1, assigned_worker_id = $2, worker_assigned_at = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query, status, workerID, workedAssignAt, taskID)
	return err
}

func (r *Repo) UpdateTaskCompletion(ctx context.Context, taskID, status string, completedAt *time.Time) error {
	query := `
		UPDATE tasks
		SET status = $1, completed_at = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, status, completedAt, taskID)
	return err
}

func (r *Repo) ScheduleTaskRetry(ctx context.Context, taskID string, retryCount int, retryAt *time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE tasks
		SET retry_count = $1, status = 'PENDING', assigned_worker_id = NULL
		WHERE id = $2, 
	`
	_, err = r.db.ExecContext(ctx, query, retryCount, taskID)
	if err != nil {
		return err
	}

	score := float64(5*1000000) + float64(retryAt.Unix())
	err = r.rd.ZAdd(ctx, "task_queue", redis.Z{
		Score: score,
		Member: taskID,
	}).Err()
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repo) RequeTask(ctx context.Context, task *models.Task) error {
  score := float64(task.Priority * 1000000) + float64(time.Now().Unix()) 

  err := r.rd.ZAdd(ctx, "task_queue", redis.Z{
    Score: score,
    Member: task.ID,
  }).Err()
  return err
}

func ConnectDB(cdf config.DatabaseConfig) *sql.DB {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cdf.Username, cdf.Password, cdf.Host, cdf.Port, cdf.Name,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Erorr Opening Database: %v", err)
	}

	db.SetMaxOpenConns(cdf.MaxConnLifetime)
	db.SetMaxIdleConns(cdf.MaxConnIdleTime)
	db.SetConnMaxIdleTime(time.Duration(cdf.MaxIdleConnection) * time.Second)
	db.SetConnMaxLifetime(time.Duration(cdf.MaxConnLifetime) * time.Second)

	if err := db.Ping(); err != nil {
		log.Fatalf("Erorr Opening Database: %v", err)
	}
	log.Println("Connected to Postgres successfully")
	return db
}

