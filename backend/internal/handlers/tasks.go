package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	httperr "taskflow/backend/internal/errors"
	"taskflow/backend/internal/middleware"
	"taskflow/backend/internal/models"
	"taskflow/backend/internal/realtime"
	"taskflow/backend/internal/store"
)

type TasksHandler struct {
	Store *store.Store
	Bus   *realtime.Hub
}

func (h *TasksHandler) publishProjectTasks(projectID uuid.UUID) {
	if h.Bus != nil {
		h.Bus.PublishProjectTasks(projectID)
	}
}

type taskCreateReq struct {
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	Priority    string     `json:"priority"`
	AssigneeID  *uuid.UUID `json:"assignee_id"`
	DueDate     *string    `json:"due_date"`
}

func validStatus(s string) bool {
	switch models.TaskStatus(s) {
	case models.TaskTodo, models.TaskInProgress, models.TaskDone:
		return true
	default:
		return false
	}
}

func validPriority(p string) bool {
	switch models.TaskPriority(p) {
	case models.PriorityLow, models.PriorityMedium, models.PriorityHigh:
		return true
	default:
		return false
	}
}

func (h *TasksHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	okAccess, err := h.Store.CanAccessProject(r.Context(), uid, projectID)
	if err != nil || !okAccess {
		if err != nil {
			httperr.Internal(w, "database error")
			return
		}
		httperr.NotFound(w)
		return
	}
	var status *string
	if v := r.URL.Query().Get("status"); v != "" {
		if !validStatus(v) {
			httperr.Validation(w, map[string]string{"status": "must be todo, in_progress, or done"})
			return
		}
		status = &v
	}
	var assignee *uuid.UUID
	if v := r.URL.Query().Get("assignee"); v != "" {
		a, err := uuid.Parse(v)
		if err != nil {
			httperr.Validation(w, map[string]string{"assignee": "must be a valid UUID"})
			return
		}
		assignee = &a
	}
	offset, limit := parsePagination(r)
	tasks, err := h.Store.ListTasks(r.Context(), projectID, status, assignee, offset, limit)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (h *TasksHandler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	okAccess, err := h.Store.CanAccessProject(r.Context(), uid, projectID)
	if err != nil || !okAccess {
		if err != nil {
			httperr.Internal(w, "database error")
			return
		}
		httperr.NotFound(w)
		return
	}
	var body taskCreateReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httperr.Validation(w, map[string]string{"_body": "invalid JSON"})
		return
	}
	fields := map[string]string{}
	if v := validateRequired("title", body.Title); v != "" {
		fields["title"] = v
	}
	pri := strings.TrimSpace(body.Priority)
	if pri == "" {
		pri = string(models.PriorityMedium)
	}
	if !validPriority(pri) {
		fields["priority"] = "must be low, medium, or high"
	}
	if len(fields) > 0 {
		httperr.Validation(w, fields)
		return
	}
	if body.AssigneeID != nil {
		okA, err := h.Store.CanBeAssignee(r.Context(), projectID, *body.AssigneeID)
		if err != nil {
			httperr.Internal(w, "database error")
			return
		}
		if !okA {
			httperr.Validation(w, map[string]string{"assignee_id": "user is not a member of this project"})
			return
		}
	}
	t, err := h.Store.CreateTask(
		r.Context(),
		strings.TrimSpace(body.Title),
		body.Description,
		models.TaskTodo,
		models.TaskPriority(pri),
		projectID,
		uid,
		body.AssigneeID,
		body.DueDate,
	)
	if err != nil {
		if strings.Contains(err.Error(), "due_date") {
			httperr.Validation(w, map[string]string{"due_date": "must be YYYY-MM-DD"})
			return
		}
		httperr.Internal(w, "could not create task")
		return
	}
	h.publishProjectTasks(projectID)
	httperr.WriteJSON(w, http.StatusCreated, t)
}

// Custom JSON so we can tell null (clear) from omitted (leave as-is); *uuid.UUID can't.
type assigneePatch struct {
	Set    bool
	IsNull bool
	ID     *uuid.UUID
}

func (a *assigneePatch) UnmarshalJSON(data []byte) error {
	a.Set = true
	if string(data) == "null" {
		a.IsNull = true
		return nil
	}
	var id uuid.UUID
	if err := json.Unmarshal(data, &id); err != nil {
		return err
	}
	a.ID = &id
	return nil
}

type taskPatchBody struct {
	Title       *string          `json:"title"`
	Description *string          `json:"description"`
	Status      *string          `json:"status"`
	Priority    *string          `json:"priority"`
	AssigneeID  assigneePatch    `json:"assignee_id"`
	DueDate     *json.RawMessage `json:"due_date"`
}

func (h *TasksHandler) Patch(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	taskID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	cur, err := h.Store.TaskByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w)
			return
		}
		httperr.Internal(w, "database error")
		return
	}
	owner, err := h.Store.IsProjectOwner(r.Context(), uid, cur.ProjectID)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	canEdit := owner || uid == cur.CreatedBy || (cur.AssigneeID != nil && *cur.AssigneeID == uid)
	if !canEdit {
		httperr.Forbidden(w)
		return
	}
	var body taskPatchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httperr.Validation(w, map[string]string{"_body": "invalid JSON"})
		return
	}
	next := *cur
	if body.Title != nil {
		if strings.TrimSpace(*body.Title) == "" {
			httperr.Validation(w, map[string]string{"title": "cannot be empty"})
			return
		}
		next.Title = strings.TrimSpace(*body.Title)
	}
	if body.Description != nil {
		next.Description = body.Description
	}
	if body.Status != nil {
		if !validStatus(*body.Status) {
			httperr.Validation(w, map[string]string{"status": "must be todo, in_progress, or done"})
			return
		}
		next.Status = models.TaskStatus(*body.Status)
	}
	if body.Priority != nil {
		if !validPriority(*body.Priority) {
			httperr.Validation(w, map[string]string{"priority": "must be low, medium, or high"})
			return
		}
		next.Priority = models.TaskPriority(*body.Priority)
	}
	if body.AssigneeID.Set {
		if body.AssigneeID.IsNull {
			next.AssigneeID = nil
		} else if body.AssigneeID.ID != nil {
			id := *body.AssigneeID.ID
			okA, err := h.Store.CanBeAssignee(r.Context(), cur.ProjectID, id)
			if err != nil {
				httperr.Internal(w, "database error")
				return
			}
			if !okA {
				httperr.Validation(w, map[string]string{"assignee_id": "user is not a member of this project"})
				return
			}
			next.AssigneeID = &id
		}
	}
	if body.DueDate != nil {
		raw := strings.TrimSpace(string(*body.DueDate))
		if raw == "null" || raw == `""` {
			next.DueDate = nil
		} else {
			var s string
			if err := json.Unmarshal(*body.DueDate, &s); err != nil {
				httperr.Validation(w, map[string]string{"due_date": "must be YYYY-MM-DD or null"})
				return
			}
			next.DueDate = &s
		}
	}
	if next.Status != cur.Status {
		so, err := h.Store.NextSortOrder(r.Context(), cur.ProjectID, next.Status)
		if err != nil {
			httperr.Internal(w, "database error")
			return
		}
		next.SortOrder = so
	}
	out, err := h.Store.SaveTask(r.Context(), &next)
	if err != nil {
		if strings.Contains(err.Error(), "due_date") {
			httperr.Validation(w, map[string]string{"due_date": "must be YYYY-MM-DD"})
			return
		}
		httperr.Internal(w, "could not update task")
		return
	}
	h.publishProjectTasks(cur.ProjectID)
	httperr.WriteJSON(w, http.StatusOK, out)
}

func (h *TasksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	taskID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	cur, err := h.Store.TaskByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w)
			return
		}
		httperr.Internal(w, "database error")
		return
	}
	owner, err := h.Store.IsProjectOwner(r.Context(), uid, cur.ProjectID)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	if !owner && uid != cur.CreatedBy {
		httperr.Forbidden(w)
		return
	}
	if err := h.Store.DeleteTask(r.Context(), taskID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w)
			return
		}
		httperr.Internal(w, "could not delete task")
		return
	}
	h.publishProjectTasks(cur.ProjectID)
	w.WriteHeader(http.StatusNoContent)
}

type reorderBody struct {
	Columns map[string][]uuid.UUID `json:"columns"`
}

func (h *TasksHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	okAccess, err := h.Store.CanAccessProject(r.Context(), uid, projectID)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	if !okAccess {
		httperr.NotFound(w)
		return
	}
	var body reorderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httperr.Validation(w, map[string]string{"_body": "invalid JSON"})
		return
	}
	if body.Columns == nil {
		httperr.Validation(w, map[string]string{"columns": "required"})
		return
	}
	tasks, err := h.Store.TasksByProject(r.Context(), projectID)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	if err := validateReorderTasks(tasks, body.Columns); err != nil {
		httperr.Validation(w, map[string]string{"columns": err.Error()})
		return
	}
	if err := h.Store.ReorderTasks(r.Context(), projectID, body.Columns); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w)
			return
		}
		httperr.Internal(w, "could not reorder")
		return
	}
	h.publishProjectTasks(projectID)
	w.WriteHeader(http.StatusNoContent)
}

func validateReorderTasks(tasks []models.Task, columns map[string][]uuid.UUID) error {
	want := make(map[uuid.UUID]struct{})
	for _, t := range tasks {
		want[t.ID] = struct{}{}
	}
	seen := make(map[uuid.UUID]struct{})
	for status, ids := range columns {
		if status != string(models.TaskTodo) && status != string(models.TaskInProgress) && status != string(models.TaskDone) {
			return errors.New("invalid status key")
		}
		for _, id := range ids {
			if _, ok := want[id]; !ok {
				return errors.New("unknown task id")
			}
			if _, dup := seen[id]; dup {
				return errors.New("duplicate task id")
			}
			seen[id] = struct{}{}
		}
	}
	if len(seen) != len(want) {
		return errors.New("must include every task exactly once")
	}
	return nil
}
