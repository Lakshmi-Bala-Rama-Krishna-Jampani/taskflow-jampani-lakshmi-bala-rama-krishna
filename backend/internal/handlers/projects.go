package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	httperr "taskflow/backend/internal/errors"
	"taskflow/backend/internal/middleware"
	"taskflow/backend/internal/models"
	"taskflow/backend/internal/store"
)

type ProjectsHandler struct {
	Store *store.Store
}

type projectCreateReq struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type projectPatchReq struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func parsePagination(r *http.Request) (offset, limit int) {
	limit = 50
	page := 1
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
			if limit > 100 {
				limit = 100
			}
		}
	}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	offset = (page - 1) * limit
	return offset, limit
}

func (h *ProjectsHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	offset, limit := parsePagination(r)
	projects, err := h.Store.ListProjectsPaged(r.Context(), uid, offset, limit)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	if projects == nil {
		projects = []models.Project{}
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (h *ProjectsHandler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	var body projectCreateReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httperr.Validation(w, map[string]string{"_body": "invalid JSON"})
		return
	}
	fields := map[string]string{}
	if v := validateRequired("name", body.Name); v != "" {
		fields["name"] = v
	}
	if len(fields) > 0 {
		httperr.Validation(w, fields)
		return
	}
	p, err := h.Store.CreateProject(r.Context(), body.Name, body.Description, uid)
	if err != nil {
		httperr.Internal(w, "could not create project")
		return
	}
	httperr.WriteJSON(w, http.StatusCreated, p)
}

func (h *ProjectsHandler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	okAccess, err := h.Store.CanAccessProject(r.Context(), uid, id)
	if err != nil || !okAccess {
		if err != nil {
			httperr.Internal(w, "database error")
			return
		}
		httperr.NotFound(w)
		return
	}
	p, err := h.Store.ProjectByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w)
			return
		}
		httperr.Internal(w, "database error")
		return
	}
	tasks, err := h.Store.TasksByProject(r.Context(), id)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"owner_id":    p.OwnerID,
		"created_at":  p.CreatedAt,
		"tasks":       tasks,
	})
}

func (h *ProjectsHandler) Patch(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	owner, err := h.Store.IsProjectOwner(r.Context(), uid, id)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	if !owner {
		httperr.Forbidden(w)
		return
	}
	var body projectPatchReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httperr.Validation(w, map[string]string{"_body": "invalid JSON"})
		return
	}
	if body.Name != nil && *body.Name == "" {
		httperr.Validation(w, map[string]string{"name": "cannot be empty"})
		return
	}
	p, err := h.Store.UpdateProject(r.Context(), id, body.Name, body.Description)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w)
			return
		}
		httperr.Internal(w, "could not update")
		return
	}
	httperr.WriteJSON(w, http.StatusOK, p)
}

func (h *ProjectsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	owner, err := h.Store.IsProjectOwner(r.Context(), uid, id)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	if !owner {
		httperr.Forbidden(w)
		return
	}
	if err := h.Store.DeleteProject(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w)
			return
		}
		httperr.Internal(w, "could not delete")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectsHandler) Members(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	okAccess, err := h.Store.CanAccessProject(r.Context(), uid, id)
	if err != nil || !okAccess {
		if err != nil {
			httperr.Internal(w, "database error")
			return
		}
		httperr.NotFound(w)
		return
	}
	members, err := h.Store.ListProjectMembers(r.Context(), id)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	if members == nil {
		members = []store.ProjectMember{}
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (h *ProjectsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserID(r.Context())
	if !ok {
		httperr.Unauthorized(w)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	okAccess, err := h.Store.CanAccessProject(r.Context(), uid, id)
	if err != nil || !okAccess {
		if err != nil {
			httperr.Internal(w, "database error")
			return
		}
		httperr.NotFound(w)
		return
	}
	byStatus, err := h.Store.StatsByStatus(r.Context(), id)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	byAssignee, err := h.Store.StatsByAssignee(r.Context(), id)
	if err != nil {
		httperr.Internal(w, "database error")
		return
	}
	type assigneeRow struct {
		AssigneeID *uuid.UUID `json:"assignee_id"`
		Count      int64      `json:"count"`
	}
	var rows []assigneeRow
	for _, a := range byAssignee {
		rows = append(rows, assigneeRow{AssigneeID: a.AssigneeID, Count: a.Count})
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{
		"by_status":   byStatus,
		"by_assignee": rows,
	})
}
