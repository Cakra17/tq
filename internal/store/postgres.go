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
			started_at, completed_at, retry_count, max_retries,
			assigned_worker_id, worked_assigned_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13
		)
	`

	_, err = r.db.ExecContext(ctx, query, 
		t.ID, t.Type, t.Status, t.Priority, t.UserID, t.ResourceID, t.Config,
		t.StartedAt, t.CompletedAt, t.RetryCount, t.MaxRetries,
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

