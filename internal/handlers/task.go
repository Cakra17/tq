package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/cakra17/tq/internal/models"
	"github.com/cakra17/tq/internal/store"
	"github.com/google/uuid"
)

type TaskHandler struct {
	repo store.Repo
	logger *slog.Logger
}

func NewTaskHandler(rp store.Repo, lg *slog.Logger) *TaskHandler {
	return &TaskHandler{ repo: rp, logger: lg}
}

func (t *TaskHandler) Add(w http.ResponseWriter, r *http.Request) {
	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		return
	}

	if task.Type == "" {
		http.Error(w, "Task type is required", http.StatusBadRequest)
		return
	}

	now := time.Now()
	task.ID = uuid.Must(uuid.NewV7()).String()
	task.Status = models.STATUSQUEUED
	task.CreatedAt = &now
	if task.Priority == 0 {
		task.Priority = 5
	}

	if task.MaxRetries == 0 {
		task.MaxRetries = 5
	}

	ctx := context.Background()
	err := t.repo.AddTask(ctx, task)
	if err != nil {
		t.logger.Error("Failed to add task", "error", err)
		http.Error(w, "Failed to add task", http.StatusInternalServerError)
		return
	}

	t.logger.Info("Task created succesfully", "task_id", task.ID, "type", task.Type)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}