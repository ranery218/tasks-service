package middleware

import (
	"context"
	"net/http"
	"strings"

	"tasks-service/internal/domain/entities"
	jwtservice "tasks-service/internal/infrastructure/jwt"
)

type contextKey string

const (
	ContextUserIDKey  contextKey = "user_id"
	ContextIsAdminKey contextKey = "is_admin"
)

type Auth struct {
	jwt *jwtservice.Service
}

func NewAuth(jwt *jwtservice.Service) *Auth {
	return &Auth{jwt: jwt}
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := a.jwt.ParseAccessToken(parts[1])
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ContextUserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, ContextIsAdminKey, claims.IsAdmin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ActorFromContext(ctx context.Context) (entities.Actor, bool) {
	userID, ok := ctx.Value(ContextUserIDKey).(int64)
	if !ok || userID <= 0 {
		return entities.Actor{}, false
	}

	isAdmin, ok := ctx.Value(ContextIsAdminKey).(bool)
	if !ok {
		return entities.Actor{}, false
	}

	return entities.Actor{UserID: userID, IsAdmin: isAdmin}, true
}
