package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskflow/backend/internal/models"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateUser(ctx context.Context, name, email, passwordHash string) (*models.User, error) {
	id := uuid.New()
	const q = `
		INSERT INTO users (id, name, email, password_hash, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, name, email, created_at`
	var u models.User
	err := s.pool.QueryRow(ctx, q, id, name, email, passwordHash).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*models.UserWithPassword, error) {
	const q = `SELECT id, name, email, password_hash, created_at FROM users WHERE email = $1`
	var u models.UserWithPassword
	err := s.pool.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.User.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `SELECT id, name, email, created_at FROM users WHERE id = $1`
	var u models.User
	err := s.pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func (s *Store) ListProjects(ctx context.Context, userID uuid.UUID) ([]models.Project, error) {
	return s.ListProjectsPaged(ctx, userID, 0, 1000)
}

func (s *Store) ListProjectsPaged(ctx context.Context, userID uuid.UUID, offset, limit int) ([]models.Project, error) {
	const q = `
		SELECT DISTINCT p.id, p.name, p.description, p.owner_id, p.created_at
		FROM projects p
		LEFT JOIN tasks t ON t.project_id = p.id AND t.assignee_id = $1
		WHERE p.owner_id = $1 OR t.id IS NOT NULL
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := s.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateProject(ctx context.Context, name string, description *string, ownerID uuid.UUID) (*models.Project, error) {
	id := uuid.New()
	const q = `
		INSERT INTO projects (id, name, description, owner_id, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, name, description, owner_id, created_at`
	var p models.Project
	err := s.pool.QueryRow(ctx, q, id, name, description, ownerID).Scan(
		&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ProjectByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	const q = `SELECT id, name, description, owner_id, created_at FROM projects WHERE id = $1`
	var p models.Project
	err := s.pool.QueryRow(ctx, q, id).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *Store) UpdateProject(ctx context.Context, id uuid.UUID, name *string, description *string) (*models.Project, error) {
	const q = `
		UPDATE projects SET
			name = COALESCE($2, name),
			description = COALESCE($3, description)
		WHERE id = $1
		RETURNING id, name, description, owner_id, created_at`
	var p models.Project
	err := s.pool.QueryRow(ctx, q, id, name, description).Scan(
		&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *Store) DeleteProject(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CanAccessProject(ctx context.Context, userID, projectID uuid.UUID) (bool, error) {
	owner, err := s.IsProjectOwner(ctx, userID, projectID)
	if err != nil {
		return false, err
	}
	if owner {
		return true, nil
	}
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM tasks t WHERE t.project_id = $1 AND t.assignee_id = $2
		)`
	var ok bool
	err = s.pool.QueryRow(ctx, q, projectID, userID).Scan(&ok)
	return ok, err
}

func (s *Store) IsProjectOwner(ctx context.Context, userID, projectID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM projects WHERE id = $2 AND owner_id = $1)`
	var ok bool
	err := s.pool.QueryRow(ctx, q, userID, projectID).Scan(&ok)
	return ok, err
}

func (s *Store) CanBeAssignee(ctx context.Context, projectID, userID uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS (SELECT 1 FROM projects WHERE id = $1 AND owner_id = $2)
		OR EXISTS (
			SELECT 1 FROM tasks WHERE project_id = $1 AND (assignee_id = $2 OR created_by = $2)
		)`
	var ok bool
	err := s.pool.QueryRow(ctx, q, projectID, userID).Scan(&ok)
	return ok, err
}

func (s *Store) ListTasks(ctx context.Context, projectID uuid.UUID, status *string, assignee *uuid.UUID, offset, limit int) ([]models.Task, error) {
	q := `
		SELECT id, title, description, status, priority, project_id, sort_order, assignee_id, created_by, due_date::text, created_at, updated_at
		FROM tasks WHERE project_id = $1`
	args := []any{projectID}
	n := 2
	if status != nil && *status != "" {
		q += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, *status)
		n++
	}
	if assignee != nil {
		q += fmt.Sprintf(" AND assignee_id = $%d", n)
		args = append(args, *assignee)
		n++
	}
	q += fmt.Sprintf(" ORDER BY sort_order ASC, created_at ASC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func scanTasks(rows pgx.Rows) ([]models.Task, error) {
	var out []models.Task
	for rows.Next() {
		var t models.Task
		var desc *string
		var assignee *uuid.UUID
		var due *string
		if err := rows.Scan(
			&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &t.ProjectID, &t.SortOrder, &assignee, &t.CreatedBy, &due, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		t.Description = desc
		t.AssigneeID = assignee
		t.DueDate = due
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) TasksByProject(ctx context.Context, projectID uuid.UUID) ([]models.Task, error) {
	return s.ListTasks(ctx, projectID, nil, nil, 0, 10000)
}

func (s *Store) NextSortOrder(ctx context.Context, projectID uuid.UUID, status models.TaskStatus) (int, error) {
	const q = `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM tasks WHERE project_id = $1 AND status = $2`
	var n int
	err := s.pool.QueryRow(ctx, q, projectID, string(status)).Scan(&n)
	return n, err
}

func (s *Store) ReorderTasks(ctx context.Context, projectID uuid.UUID, columns map[string][]uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for status, ids := range columns {
		if status != string(models.TaskTodo) && status != string(models.TaskInProgress) && status != string(models.TaskDone) {
			return fmt.Errorf("invalid status column")
		}
		for i, tid := range ids {
			tag, err := tx.Exec(ctx, `
				UPDATE tasks SET status = $1, sort_order = $2
				WHERE id = $3 AND project_id = $4`,
				status, i, tid, projectID)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrNotFound
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) CreateTask(ctx context.Context, title string, description *string, status models.TaskStatus, priority models.TaskPriority, projectID, createdBy uuid.UUID, assigneeID *uuid.UUID, dueDate *string) (*models.Task, error) {
	id := uuid.New()
	sortOrder, err := s.NextSortOrder(ctx, projectID, status)
	if err != nil {
		return nil, err
	}
	var due interface{}
	if dueDate != nil && *dueDate != "" {
		d, err := time.Parse("2006-01-02", *dueDate)
		if err != nil {
			return nil, fmt.Errorf("due_date: %w", err)
		}
		due = d
	}
	const q = `
		INSERT INTO tasks (id, title, description, status, priority, project_id, sort_order, assignee_id, created_by, due_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::date, NOW(), NOW())
		RETURNING id, title, description, status, priority, project_id, sort_order, assignee_id, created_by, due_date::text, created_at, updated_at`
	var t models.Task
	var desc *string
	var assignee *uuid.UUID
	var dueOut *string
	err = s.pool.QueryRow(ctx, q, id, title, description, string(status), string(priority), projectID, sortOrder, assigneeID, createdBy, due).Scan(
		&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &t.ProjectID, &t.SortOrder, &assignee, &t.CreatedBy, &dueOut, &t.CreatedAt, &t.UpdatedAt,
	)
	t.Description = desc
	t.AssigneeID = assignee
	t.DueDate = dueOut
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) TaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	const q = `
		SELECT id, title, description, status, priority, project_id, sort_order, assignee_id, created_by, due_date::text, created_at, updated_at
		FROM tasks WHERE id = $1`
	var t models.Task
	var desc *string
	var assignee *uuid.UUID
	var due *string
	err := s.pool.QueryRow(ctx, q, id).Scan(
		&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &t.ProjectID, &t.SortOrder, &assignee, &t.CreatedBy, &due, &t.CreatedAt, &t.UpdatedAt,
	)
	t.Description = desc
	t.AssigneeID = assignee
	t.DueDate = due
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

func (s *Store) SaveTask(ctx context.Context, t *models.Task) (*models.Task, error) {
	var due interface{}
	if t.DueDate != nil && *t.DueDate != "" {
		d, err := time.Parse("2006-01-02", *t.DueDate)
		if err != nil {
			return nil, fmt.Errorf("due_date: %w", err)
		}
		due = d
	}
	const q = `
		UPDATE tasks SET
			title = $2,
			description = $3,
			status = $4,
			priority = $5,
			assignee_id = $6,
			due_date = $7::date,
			sort_order = $8
		WHERE id = $1
		RETURNING id, title, description, status, priority, project_id, sort_order, assignee_id, created_by, due_date::text, created_at, updated_at`
	var out models.Task
	var desc *string
	var assignee *uuid.UUID
	var dueOut *string
	err := s.pool.QueryRow(ctx, q, t.ID, t.Title, t.Description, string(t.Status), string(t.Priority), t.AssigneeID, due, t.SortOrder).Scan(
		&out.ID, &out.Title, &desc, &out.Status, &out.Priority, &out.ProjectID, &out.SortOrder, &assignee, &out.CreatedBy, &dueOut, &out.CreatedAt, &out.UpdatedAt,
	)
	out.Description = desc
	out.AssigneeID = assignee
	out.DueDate = dueOut
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) DeleteTask(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type ProjectMember struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

func (s *Store) ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error) {
	const q = `
		SELECT DISTINCT u.id, u.name, u.email FROM users u
		WHERE u.id = (SELECT owner_id FROM projects WHERE id = $1)
		OR u.id IN (SELECT assignee_id FROM tasks WHERE project_id = $1 AND assignee_id IS NOT NULL)
		OR u.id IN (SELECT created_by FROM tasks WHERE project_id = $1)
		ORDER BY u.name`
	rows, err := s.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectMember
	for rows.Next() {
		var m ProjectMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Email); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) ProjectMemberIDs(ctx context.Context, projectID uuid.UUID) ([]uuid.UUID, error) {
	const q = `
		SELECT DISTINCT u.id FROM users u
		WHERE u.id IN (SELECT owner_id FROM projects WHERE id = $1)
		UNION
		SELECT assignee_id FROM tasks WHERE project_id = $1 AND assignee_id IS NOT NULL
		UNION
		SELECT created_by FROM tasks WHERE project_id = $1`
	rows, err := s.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) StatsByStatus(ctx context.Context, projectID uuid.UUID) (map[string]int64, error) {
	const q = `
		SELECT status, COUNT(*) FROM tasks WHERE project_id = $1 GROUP BY status`
	rows, err := s.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]int64{"todo": 0, "in_progress": 0, "done": 0}
	for rows.Next() {
		var st string
		var c int64
		if err := rows.Scan(&st, &c); err != nil {
			return nil, err
		}
		m[st] = c
	}
	return m, rows.Err()
}

func (s *Store) StatsByAssignee(ctx context.Context, projectID uuid.UUID) ([]struct {
	AssigneeID *uuid.UUID
	Count      int64
}, error) {
	const q = `
		SELECT assignee_id, COUNT(*) FROM tasks WHERE project_id = $1 GROUP BY assignee_id`
	rows, err := s.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		AssigneeID *uuid.UUID
		Count      int64
	}
	for rows.Next() {
		var a *uuid.UUID
		var c int64
		if err := rows.Scan(&a, &c); err != nil {
			return nil, err
		}
		out = append(out, struct {
			AssigneeID *uuid.UUID
			Count      int64
		}{AssigneeID: a, Count: c})
	}
	return out, rows.Err()
}
