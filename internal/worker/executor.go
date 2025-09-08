package worker

import (
  "context"
  "encoding/json"
  "fmt"
  "log/slog"
  "time"

  "github.com/cakra17/tq/internal/models"
)

type DefaultExecutor struct {
  logger *slog.Logger
}

func NewDefaultExecutor(logger *slog.Logger) *DefaultExecutor {
  return &DefaultExecutor{logger: logger}
}

func (e *DefaultExecutor) Execute(ctx context.Context, task *models.Task) error {
  e.logger.Info("Executing task", "task_id", task.ID, "type", task.Type)

  switch task.Type {
  case "send_email":
      return e.sendEmail(ctx, task) 
  default:
      return fmt.Errorf("unknown task type: %s", task.Type)
  }
}

func (e *DefaultExecutor) sendEmail(ctx context.Context, task *models.Task) error {
  var config map[string]any
  cfg, err := task.Config.Value()
  if err != nil {
    return err
  }
  if err := json.Unmarshal(cfg, &config); err != nil {
      return fmt.Errorf("invalid email config: %w", err)
  }

  // Extract email parameters
  to, ok := config["to"].(string)
  if !ok {
      return fmt.Errorf("missing 'to' field in email config")
  }

  subject, ok := config["subject"].(string)
  if !ok {
      return fmt.Errorf("missing 'subject' field in email config")
  }

  // Simulate email sending
  e.logger.Info("Sending email", "to", to, "subject", subject, "task_id", task.ID)
  
  // Simulate some work
  time.Sleep(2 * time.Second)

  e.logger.Info("Email sent successfully", "to", to, "task_id", task.ID)
  return nil
}

