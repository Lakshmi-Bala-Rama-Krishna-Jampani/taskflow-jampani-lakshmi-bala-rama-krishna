package errors

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func Validation(w http.ResponseWriter, fields map[string]string) {
	WriteJSON(w, http.StatusBadRequest, map[string]any{
		"error":  "validation failed",
		"fields": fields,
	})
}

func Unauthorized(w http.ResponseWriter) {
	WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
}

func Forbidden(w http.ResponseWriter) {
	WriteJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
}

func NotFound(w http.ResponseWriter) {
	WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func Internal(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": msg})
}
