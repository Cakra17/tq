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
  db := store.ConnectDB(cfg.Database)
	rd := store.NewRedisClient(cfg.Redis)

  repo := store.NewRepo(db, rd)

  executor := worker.NewDefaultExecutor(logger)

  w := worker.NewWorker(repo, logger, executor, 5)

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

  if err := w.Start(ctx); err != nil {
    log.Fatalf("Worker failed: %v", err)
  }

  logger.Info("Worker shutdown gracefully")
}
