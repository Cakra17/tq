package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	ctx := context.Background()
	closed := make(chan struct{})

  go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		signal := <-sigint
		
		log.Printf("Received %s signal, shutting down server", signal.String())
		shutdownCtx, cancel := context.WithTimeout(ctx, 5 * time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}

		close(closed)
	}()

	log.Printf("server running on port %s", server.Addr[1:])
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Failed to run server: %v", err)
	}

	<-closed
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
