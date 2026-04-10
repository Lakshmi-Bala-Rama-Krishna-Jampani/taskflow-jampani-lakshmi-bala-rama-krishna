DROP INDEX IF EXISTS idx_tasks_project_status_sort;
ALTER TABLE tasks DROP COLUMN IF EXISTS sort_order;
