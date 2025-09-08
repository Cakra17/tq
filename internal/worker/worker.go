package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cakra17/tq/internal/models"
	"github.com/cakra17/tq/internal/store"
	"github.com/google/uuid"
)

type Worker struct {
	ID   string
	repo store.Repo
	logger *slog.Logger
	executor *DefaultExecutor
	concurency int
	stopCh chan struct{}
	wg sync.WaitGroup
}

type TaskExecutor interface {
	Execute(ctx context.Context, task models.Task) error
}

func NewWorker(repo store.Repo, logger *slog.Logger, executor *DefaultExecutor, concurency int) *Worker {
	if concurency <= 0 {
		concurency = 5
	}

	return &Worker{
		ID: fmt.Sprintf("worker_%s", uuid.NewString()[:8]),
		repo: repo,
		logger: logger,
		executor: executor,
		concurency: concurency,
		stopCh: make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	taskCh := make(chan *models.Task, w.concurency*2)
	
	for i := range w.concurency {
		w.wg.Add(1)
		go w.taskWorker(ctx, taskCh, i)
	}

	w.wg.Add(1)
	go w.taskFetcher(ctx, taskCh)

	<-w.stopCh
	w.logger.Info("Worker shutting down", "worked_id", w.ID)

	close(w.stopCh)

	w.wg.Wait()

	w.logger.Info("Worker shutdown gracefully", "worker_id", w.ID)
	return nil
}

func (w *Worker) Stop() {
	close(w.stopCh)
}

func (w *Worker) taskFetcher(ctx context.Context, taskCh chan <- *models.Task) {
	defer w.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			for {
				task, err := w.repo.PullNextTask(ctx, w.ID)
				if err != nil {
					w.logger.Error("Error pulling task", "error", err)
					break
				}

				if task == nil {
					break
				}

				select {
				case taskCh <- task:

				// todo
				case <-ctx.Done():
          w.repo.RequeTask(ctx, task)
          return
				case <-w.stopCh:
          w.repo.RequeTask(ctx, task) 
          return
				default:
					break
				}
			}
		}
	}
}

func (w *Worker) taskWorker(ctx context.Context, taskCh <- chan *models.Task, workerNum int) {
	defer w.wg.Done()

	workerLogger := w.logger.With("worker_id", w.ID, "goroutine", workerNum)
	workerLogger.Info("Task worker started")

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-taskCh:
			if !ok {
				workerLogger.Info("Task worker is shutting down")
				return
			}

			w.processTask(ctx, task, workerLogger)
		}
	}
}

func (w *Worker) processTask(ctx context.Context, task *models.Task, logger *slog.Logger) {
	logger.Info("Processing task",
		"task_id", task.ID,
		"task_type", task.Type,
	)

	startTime := time.Now()

	execErr := w.executor.Execute(ctx, *task)
	duration := time.Since(startTime)

	if execErr != nil {
		w.handleTaskFailure(ctx, task, execErr)
	} else {
    w.handleTaskSuccess(ctx, task, duration)
	}
}

func (w *Worker) handleTaskSuccess(ctx context.Context, task *models.Task, duration time.Duration) error {
	now := time.Now()
	task.Status = "COMPLETED"
	task.CompletedAt = &now
	
	err := w.repo.UpdateTaskCompletion(ctx, task.ID, "COMPLETED", &now)
	if err != nil {
			w.logger.Error("Failed to update completed task", 
					"task_id", task.ID, "error", err)
			return err
	}
	
	w.logger.Info("Task completed successfully", 
			"task_id", task.ID, 
			"duration", duration,
			"worker_id", w.ID)
	
	return nil
}

func (w *Worker) handleTaskFailure(ctx context.Context, task *models.Task, execErr error) error {
	w.logger.Error("Task execution failed", 
			"task_id", task.ID, 
			"error", execErr,
			"retry_count", task.RetryCount,
			"max_retries", task.MaxRetries)
	
	// Check if we should retry
	if task.RetryCount < task.MaxRetries {
			return w.retryTask(ctx, task)
	}
	
	// Max retries exceeded - mark as permanently failed
	now := time.Now()
	task.Status = "FAILED"
	task.CompletedAt = &now
	
	err := w.repo.UpdateTaskCompletion(ctx, task.ID, "FAILED", &now)
	if err != nil {
			w.logger.Error("Failed to update failed task", 
					"task_id", task.ID, "error", err)
			return err
	}
	
	w.logger.Error("Task permanently failed", 
			"task_id", task.ID,
			"final_error", execErr.Error())
	
	return nil
}

func (w *Worker) retryTask(ctx context.Context, task *models.Task) error {
  newRetryCount := task.RetryCount + 1

  backOffDelay := min(time.Duration(1<<uint(newRetryCount-1)) * time.Second, 5 * time.Minute)

  retryAt := time.Now().Add(backOffDelay)

  err := w.repo.ScheduleTaskRetry(ctx, task.ID, newRetryCount, &retryAt)
  if err != nil {
    return fmt.Errorf("Failed to retry task: %v", err)
  }

  w.logger.Info("Task scheduled for retry",
    "task_id", task.ID,
    "retry_count", newRetryCount,
    "retry_at", retryAt,
  )
  return nil
}
