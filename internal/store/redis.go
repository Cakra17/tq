package store

import (
	"context"
	"log/slog"
	"time"

	"github.com/cakra17/tq/internal/config"
	. "github.com/cakra17/tq/internal/models"
	"github.com/redis/go-redis/v9"
)

type QueueService struct {
	rd *redis.Client
	lg *slog.Logger
}

func NewQueueService(cfg config.RedisConfig, lg *slog.Logger) QueueService {
	rd := redis.NewClient(&redis.Options{
		Addr: cfg.Host,
		Password: cfg.Password,
		DB: cfg.DB,
	})

	return QueueService{rd: rd, lg: lg}
}

func (s *QueueService) PushTask(ctx context.Context, task *Task) error {
	score := task.Priority * 1_000_000_000 + int(time.Now().UnixMilli())

	if err := s.rd.ZAdd(ctx, "task_queue", redis.Z{
		Score: float64(score),
		Member: task.ID,
	}).Err(); err != nil {
		s.lg.ErrorContext(ctx, "Queue Error", "Failed to add task", err.Error())
		return err
	}

	return nil
}

func (s *QueueService) PullTask(ctx context.Context) (string, error) {
	result, err := s.rd.ZPopMin(ctx, "task_queue", 1).Result()
	if err != nil {
		if err == redis.Nil {
			s.lg.ErrorContext(ctx, "Queue Error", "Failed to pull the task", err.Error())
			return "", nil
		}
		s.lg.ErrorContext(ctx, "Queue Error", "Failed to pull the task", err.Error())
		return "", err
	}

	if len(result) == 0 {
		return "", nil
	}

	taskID := result[0].Member.(string)
	return taskID, nil
}