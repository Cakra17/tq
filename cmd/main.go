package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

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
	log.Fatalln(http.ListenAndServe(":6969", mux))
}


func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		next.ServeHTTP(w, r)
	}
}
