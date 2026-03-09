package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tasks-service/internal/infrastructure/ratelimit"
)

func TestRateLimitBlocksAfterLimit(t *testing.T) {
	t.Parallel()

	store := ratelimit.NewInMemoryStore()
	rl := NewRateLimit(store, 2, time.Minute)
	rl.nowFn = func() time.Time {
		return time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	}

	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		req = req.WithContext(withActor(req.Context(), 1, false))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req = req.WithContext(withActor(req.Context(), 1, false))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestRateLimitUsesIPWhenActorMissing(t *testing.T) {
	t.Parallel()

	store := ratelimit.NewInMemoryStore()
	rl := NewRateLimit(store, 1, time.Minute)
	rl.nowFn = func() time.Time {
		return time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	}

	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	req2.RemoteAddr = "10.0.0.1:54321"
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request 429, got %d", w2.Code)
	}
}

func withActor(ctx context.Context, userID int64, isAdmin bool) context.Context {
	ctx = context.WithValue(ctx, ContextUserIDKey, userID)
	ctx = context.WithValue(ctx, ContextIsAdminKey, isAdmin)
	return ctx
}
