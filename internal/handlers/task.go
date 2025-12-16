package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	. "github.com/cakra17/tq/internal/models"
	"github.com/cakra17/tq/internal/store"
	"github.com/google/uuid"
)

type TaskHandler struct {
	tr store.TaskRepo
	lg *slog.Logger
	qs store.QueueService
}

func NewTaskHandler(tr store.TaskRepo, lg *slog.Logger, qs store.QueueService) *TaskHandler {
	return &TaskHandler{ tr: tr, lg: lg, qs: qs}
}

func (t *TaskHandler) Add(w http.ResponseWriter, r *http.Request) {
	var task Task
	ctx := r.Context()

	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		t.lg.Error("Unexpected payload", "Failed to decode", err.Error())
		SendResponse(w, http.StatusInternalServerError, Response{
			Message: "Failed to decode",
		})
		return
	}

	if task.Type == "" {
		t.lg.Error("Unexpected payload", "Task type error", "Task type is required")
		SendResponse(w, http.StatusInternalServerError, Response{
			Message: "Task Type is required",
		})
		return
	}

	now := time.Now()
	task.ID = uuid.Must(uuid.NewV7()).String()
	task.Status = STATUSQUEUED
	task.CreatedAt = &now

	if task.Priority == 0 {
		task.Priority = 3
	}

	if task.MaxRetries == 0 {
		task.MaxRetries = 5
	}

	err := t.tr.AddTask(ctx, task)
	if err != nil {
		t.lg.Error("Database Error","Failed to add task", err.Error())
		http.Error(w, "Failed to add task", http.StatusInternalServerError)
		return
	}

	err = t.qs.PushTask(ctx, &task)
	if err != nil {
		t.lg.ErrorContext(ctx, "Queue Error", "Failed to add task to queue", err.Error())
		SendResponse(w, http.StatusInternalServerError, Response{
			Message: "Failed to add task to queue",
		})
		return
	}

	SendResponse(w, http.StatusCreated, Response{
		Message: "Success to add task",
		Data: task,
	})
}

func (t *TaskHandler) GetTaskById(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	ctx := r.Context()
	task, err := t.tr.GetTask(ctx, taskID)
	if err != nil {
		SendResponse(w, http.StatusNotFound, Response{
			Message: err.Error(),
		})
		return
	}

	SendResponse(w, http.StatusOK, Response{
		Message: "Successful get task data",
		Data: *task,
	})
}