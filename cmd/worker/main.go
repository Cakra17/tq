package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/cakra17/tq/internal/config"
	"github.com/cakra17/tq/internal/store"
	"github.com/cakra17/tq/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

  cfg := config.LoadEnv()
  db := config.ConnectDB(cfg.Database)
	queueService := store.NewQueueService(cfg.Redis, logger)

  repo := store.NewTaskRepo(db, logger)

  executor := worker.NewDefaultExecutor(logger, cfg.SMTP)

  w := worker.NewWorker(repo, &queueService, logger, executor, 1)

  ctx, cancel := context.WithCancel(context.Background())
  defer cancel()

  sigCh := make(chan os.Signal, 1)
  signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

  go func() {
    <-sigCh
    logger.Info("Shutdown signal recieved")
    w.Stop()
    cancel()
  }()

  logger.Info("Worker is running")
  if err := w.Start(ctx); err != nil {
    log.Fatalf("Worker failed: %v", err)
  }

  logger.Info("Worker shutdown gracefully")
}
