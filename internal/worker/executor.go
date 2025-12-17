package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"

	"github.com/cakra17/tq/internal/config"
	"github.com/cakra17/tq/internal/models"
)

type DefaultExecutor struct {
  lg *slog.Logger
  cfg config.SMTPConfig
}

func NewDefaultExecutor(lg *slog.Logger, cfg config.SMTPConfig) *DefaultExecutor {
  return &DefaultExecutor{lg: lg, cfg: cfg}
}

func (e *DefaultExecutor) Execute(ctx context.Context, task *models.Task) error {
  e.lg.Info("Executing task", "task_id", task.ID, "type", task.Type)

  switch task.Type {
  case "send_email":
    return e.sendEmail(task) 
  default:
    return fmt.Errorf("unknown task type: %s", task.Type)
  }
}

func (e *DefaultExecutor) sendEmail(task *models.Task) error {
  var Taskconfig models.TaskConfig

  cfg, err := task.Config.Value()
  if err != nil {
    return err
  }

  if err := Taskconfig.Scan(cfg); err != nil {
    return fmt.Errorf("invalid email config: %w", err)
  }

  to, ok :=Taskconfig["to"].(string)
  if !ok {
    return fmt.Errorf("missing 'to' field in email config")
  }

  subject, ok := Taskconfig["subject"].(string)
  if !ok {
    return fmt.Errorf("missing 'subject' field in email config")
  }

  variables, ok := Taskconfig["variables"].(map[string]any)
  if !ok {
    return fmt.Errorf("missing 'variables' field in email config")
  }

  activationLink, ok := variables["activation_link"].(string)
  if !ok {
    return fmt.Errorf("missing 'activation_link' field in email config")
  }

  body := "From: " + e.cfg.SenderName + "\n" +
        "To: " + to + "\n" +
        "Cc: " + e.cfg.AuthEmail + "\n" +
        "Subject: " + subject + "\n\n" +
        "Selamat datang di platform ini, terima kasih sudah mendaftar " + activationLink

  auth := smtp.PlainAuth("", e.cfg.AuthEmail, e.cfg.AuthPassword, e.cfg.Host)
  smtpAddr := fmt.Sprintf("%s:%s", e.cfg.Host, e.cfg.Port)
  
  err = smtp.SendMail(smtpAddr, auth, e.cfg.AuthEmail, []string{to}, []byte(body))
  if err != nil {
    e.lg.Error("Failed to send email", "to", to, "task_id", task.ID, "problem", err.Error())
    return err
  }

  e.lg.Info("Email sent successfully", "to", to, "task_id", task.ID)
  return nil
}

