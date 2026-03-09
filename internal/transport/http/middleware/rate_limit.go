package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"tasks-service/internal/infrastructure/ratelimit"
)

type RateLimit struct {
	store  ratelimit.Store
	limit  int64
	window time.Duration
	nowFn  func() time.Time
}

func NewRateLimit(store ratelimit.Store, limit int64, window time.Duration) *RateLimit {
	if limit <= 0 {
		limit = 100
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimit{
		store:  store,
		limit:  limit,
		window: window,
		nowFn:  time.Now().UTC,
	}
}

func (m *RateLimit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := m.buildKey(r)
		count, err := m.store.IncrementAndGet(r.Context(), key, m.window)
		if err != nil {
			http.Error(w, "internal_error", http.StatusInternalServerError)
			return
		}

		if count > m.limit {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"too many requests"}}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *RateLimit) buildKey(r *http.Request) string {
	nowMinute := m.nowFn().UTC().Format("200601021504")

	if actor, ok := ActorFromContext(r.Context()); ok {
		return fmt.Sprintf("rate:user:%d:%s", actor.UserID, nowMinute)
	}

	ip := clientIP(r)
	return fmt.Sprintf("rate:ip:%s:%s", ip, nowMinute)
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}

	return "unknown"
}
