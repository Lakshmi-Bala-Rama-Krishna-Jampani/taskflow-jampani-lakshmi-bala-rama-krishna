package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"taskflow/backend/internal/auth"
	httperr "taskflow/backend/internal/errors"
	"taskflow/backend/internal/models"
	"taskflow/backend/internal/store"
)

type AuthHandler struct {
	Store      *store.Store
	JWTSecret  []byte
	BcryptCost int
}

type authReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResp struct {
	Token string       `json:"token"`
	User  models.User `json:"user"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body authReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httperr.Validation(w, map[string]string{"_body": "invalid JSON"})
		return
	}
	fields := map[string]string{}
	if v := validateRequired("name", body.Name); v != "" {
		fields["name"] = v
	}
	if v := validateEmail(body.Email); v != "" {
		fields["email"] = v
	}
	if v := validatePassword(body.Password); v != "" {
		fields["password"] = v
	}
	if len(fields) > 0 {
		httperr.Validation(w, fields)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), h.BcryptCost)
	if err != nil {
		httperr.Internal(w, "could not hash password")
		return
	}
	u, err := h.Store.CreateUser(r.Context(), strings.TrimSpace(body.Name), strings.TrimSpace(strings.ToLower(body.Email)), string(hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			(strings.Contains(strings.ToLower(pgErr.ConstraintName), "email") ||
				(pgErr.Detail != "" && strings.Contains(strings.ToLower(pgErr.Detail), "email"))) {
			httperr.Validation(w, map[string]string{"email": "is already registered"})
			return
		}
		httperr.Internal(w, "could not create user")
		return
	}
	token, err := auth.SignJWT(h.JWTSecret, u.ID, u.Email, 24*time.Hour)
	if err != nil {
		httperr.Internal(w, "could not sign token")
		return
	}
	httperr.WriteJSON(w, http.StatusCreated, authResp{Token: token, User: *u})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httperr.Validation(w, map[string]string{"_body": "invalid JSON"})
		return
	}
	fields := map[string]string{}
	if v := validateEmail(body.Email); v != "" {
		fields["email"] = v
	}
	if strings.TrimSpace(body.Password) == "" {
		fields["password"] = "is required"
	}
	if len(fields) > 0 {
		httperr.Validation(w, fields)
		return
	}
	u, err := h.Store.UserByEmail(r.Context(), strings.TrimSpace(strings.ToLower(body.Email)))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.Unauthorized(w)
			return
		}
		httperr.Internal(w, "database error")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(body.Password)); err != nil {
		httperr.Unauthorized(w)
		return
	}
	token, err := auth.SignJWT(h.JWTSecret, u.ID, u.Email, 24*time.Hour)
	if err != nil {
		httperr.Internal(w, "could not sign token")
		return
	}
	httperr.WriteJSON(w, http.StatusOK, authResp{Token: token, User: u.User})
}
