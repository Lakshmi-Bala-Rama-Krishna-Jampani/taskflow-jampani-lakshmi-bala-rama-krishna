ALTER TABLE tasks ADD COLUMN sort_order INT NOT NULL DEFAULT 0;

WITH ranked AS (
  SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id, status ORDER BY created_at) - 1 AS rn
  FROM tasks
)
UPDATE tasks t SET sort_order = ranked.rn FROM ranked WHERE t.id = ranked.id;

CREATE INDEX idx_tasks_project_status_sort ON tasks (project_id, status, sort_order);
