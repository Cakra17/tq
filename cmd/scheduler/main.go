package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/cakra17/tq/internal/config"
	"github.com/cakra17/tq/internal/handlers"
	"github.com/cakra17/tq/internal/store"
)

func main() {
	mux := http.NewServeMux()

	// configuration
	cfg := config.LoadEnv()
	db := store.ConnectDB(cfg.Database)
	rd := store.NewRedisClient(cfg.Redis)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// repository
	repo := store.NewRepo(db, rd)

	// handler
	taskHandler := handlers.NewTaskHandler(repo, logger)

	// endpoint
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hawo"))
	})
	mux.HandleFunc("POST /", cors(taskHandler.Add))

  server := &http.Server{
    Addr: cfg.AppPort,
    Handler: mux,
  }

  go func() {
    log.Printf("Listening on port %s", cfg.AppPort)
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
      log.Fatalf("Server failed: %v", err)
    }
  }()

  // gracefully shutdown
  sig := make(chan os.Signal, 1)
  signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
  <-sig

  if err := server.Shutdown(context.Background()); err != nil {
    log.Fatalf("Server forced to shutdown: %v", err)
  }
  log.Println("Server shutdown gracefully")
}


func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		next.ServeHTTP(w, r)
	}
}
