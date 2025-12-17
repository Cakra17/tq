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
	repo store.TaskRepo
	queue *store.QueueService
	logger *slog.Logger
	executor *DefaultExecutor
	count int
	stopCh chan struct{}
	wg sync.WaitGroup
}

func NewWorker(
	repo store.TaskRepo, queue *store.QueueService, 
	logger *slog.Logger, executor *DefaultExecutor, 
	count int,
) *Worker {
	if count <= 0 {
		count = 5
	}

	return &Worker{
		ID: fmt.Sprintf("worker_%s", uuid.NewString()[:8]),
		repo: repo,
		queue: queue,
		logger: logger,
		executor: executor,
		count: count,
		stopCh: make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	taskCh := make(chan *models.Task, w.count*2)
	
	for i := 1; i <= w.count; i++ {
		w.wg.Add(1)
		go w.taskWorker(ctx, taskCh, i)
	}

	w.wg.Add(1)
	go w.taskFetcher(ctx, taskCh)

	<-w.stopCh
	w.logger.Info("Worker shutting down", "worked_id", w.ID)

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
				taskID, err := w.queue.PullTask(ctx)
				if err != nil {
					w.logger.Error("Worker Error", "Failed to pull task from queue", err.Error())
					break
				}

				task, err := w.repo.GetTask(ctx, taskID)
				if err != nil {
					w.logger.Error("Worker Error", "Failed to get task from db", err.Error())
					break
				}

				if task == nil {
					break
				}

				select {
				case taskCh <- task:
				case <-ctx.Done():
          w.queue.PushTask(ctx, task)
          return
				case <-w.stopCh:
          w.queue.PushTask(ctx, task) 
          return
				default:
					return
				}
			}
		}
	}
}

func (w *Worker) taskWorker(ctx context.Context, taskCh <- chan *models.Task, workerNum int) {
	defer w.wg.Done()

	workerLogger := w.logger.With("worker_id", w.ID, "worker_num", workerNum)
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
			w.processTask(ctx, task)
		}
	}
}

func (w *Worker) processTask(ctx context.Context, task *models.Task) {
	startTime := time.Now()
	workerStartTime := startTime

	if err := w.repo.UpdateTaskStart(
		ctx, task.ID, w.ID, models.STATUSRUNNING, 
		task.RetryCount, &startTime, &workerStartTime,
	); err != nil {
		w.logger.Info(
			"Failed to start task",
			"task_id", task.ID,
			"task_type", task.Type,
		)
		return
	}

	w.logger.Info("Processing task",
		"task_id", task.ID,
		"task_type", task.Type,
	)

	execErr := w.executor.Execute(ctx, task)
	duration := time.Since(startTime)

	if execErr != nil {
		w.handleTaskFailure(ctx, task, execErr)
	} else {
    w.handleTaskSuccess(ctx, task, duration)
	}
}

func (w *Worker) handleTaskSuccess(ctx context.Context, task *models.Task, duration time.Duration) error {
	now := time.Now()
	task.CompletedAt = &now
	
	err := w.repo.UpdateTaskCompletion(ctx, task.ID, models.STATUSCOMPLETED, &now)
	if err != nil {
		w.logger.Error(
			"Failed to update completed task", 
			"task_id", task.ID, 
			"error", err,
		)
		return err
	}
	
	w.logger.Info("Task completed successfully", 
		"task_id", task.ID, 
		"duration", duration,
		"worker_id", w.ID,
	)
	
	return nil
}

func (w *Worker) handleTaskFailure(ctx context.Context, task *models.Task, execErr error) error {
	w.logger.Error("Task execution failed", 
		"task_id", task.ID, 
		"error", execErr,
		"retry_count", task.RetryCount,
		"max_retries", task.MaxRetries,
	)
	
	if task.CanRetry() {
		return w.retryTask(ctx, task)
	}

	now := time.Now()
	task.Status = models.STATUSFAILED
	task.CompletedAt = &now
	
	err := w.repo.UpdateTaskCompletion(ctx, task.ID, models.STATUSFAILED, &now)
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
	task.RetryCount += 1

	err := w.repo.UpdateTaskRetry(ctx, task.ID, task.RetryCount)
	if err != nil {
		return err
	}

	err = w.queue.PushTask(ctx, task)
	if err != nil {
		return err
	}

  w.logger.Info("Task scheduled for retry",
    "task_id", task.ID,
    "retry_count", task.RetryCount,
  )
  return nil
}
