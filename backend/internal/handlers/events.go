package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"taskflow/backend/internal/auth"
	httperr "taskflow/backend/internal/errors"
	"taskflow/backend/internal/realtime"
	"taskflow/backend/internal/store"
)

// SSE for task updates on a project.
type EventsHandler struct {
	Store     *store.Store
	Hub       *realtime.Hub
	JWTSecret []byte
}

func (h *EventsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httperr.Unauthorized(w)
		return
	}
	claims, err := auth.ParseJWT(h.JWTSecret, token)
	if err != nil {
		httperr.Unauthorized(w)
		return
	}
	uid := claims.UserID
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.NotFound(w)
		return
	}
	okAccess, err := h.Store.CanAccessProject(r.Context(), uid, projectID)
	if err != nil {
		slog.Error("events stream access check", "err", err, "project_id", projectID, "user_id", uid)
		httperr.Internal(w, "database error")
		return
	}
	if !okAccess {
		httperr.NotFound(w)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httperr.Internal(w, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cancel := h.Hub.Subscribe(projectID)
	defer cancel()

	fmt.Fprintf(w, ": connected\n\n")
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"sse_connected"}`)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}
