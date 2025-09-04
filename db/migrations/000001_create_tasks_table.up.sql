CREATE TABLE IF NOT EXISTS tasks (
  id UUID PRIMARY KEY,
  type VARCHAR(50) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT "PENDING",
  priority INTEGER NOT NULL DEFAULT 5,
  user_id BIGINT,
  resource_id VARCHAR(100),
  config JSONB,
  created_at TIMESTAMP DEFAULT NOW(),
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  retry_count INTEGER DEFAULT 0,
  max_retries INTEGER DEFAULT 5,
  assigned_worker_id VARCHAR(50),
  worker_assigned_at TIMESTAMP
);

CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_type_status ON tasks(type, status);
CREATE INDEX idx_tasks_priority ON tasks(priority DESC, created_at);
CREATE INDEX idx_tasks_worker ON tasks(assigned_worker_id);