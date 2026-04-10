package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"taskflow/backend/internal/auth"
	httperr "taskflow/backend/internal/errors"
)

type ctxKey int

const userIDKey ctxKey = 1

func UserID(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(userIDKey).(uuid.UUID)
	return v, ok
}

func JWT(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if h == "" || !strings.HasPrefix(strings.ToLower(h), "bearer ") {
				httperr.Unauthorized(w)
				return
			}
			raw := strings.TrimSpace(h[7:])
			claims, err := auth.ParseJWT(secret, raw)
			if err != nil {
				httperr.Unauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
