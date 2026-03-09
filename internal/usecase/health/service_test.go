package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakePinger struct {
	name string
	err  error
}

func (f fakePinger) Name() string { return f.name }
func (f fakePinger) Ping(context.Context) error {
	return f.err
}

func TestHealthReturnsOkWithTimestamp(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	now := time.Date(2026, 3, 9, 10, 0, 0, 0, time.UTC)
	svc.nowFn = func() time.Time { return now }

	got := svc.Health(context.Background())
	if got.Status != "ok" {
		t.Fatalf("expected status ok, got %s", got.Status)
	}
	if !got.CheckedAtUTC.Equal(now) {
		t.Fatalf("unexpected checked_at: %v", got.CheckedAtUTC)
	}
}

func TestReadyWithoutDependencies(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	now := time.Date(2026, 3, 9, 11, 0, 0, 0, time.UTC)
	svc.nowFn = func() time.Time { return now }

	got, ok := svc.Ready(context.Background())
	if !ok {
		t.Fatalf("expected ready=true")
	}
	if got.Status != "ready" {
		t.Fatalf("expected status ready, got %s", got.Status)
	}
	if len(got.Dependencies) != 0 {
		t.Fatalf("expected no dependencies, got %d", len(got.Dependencies))
	}
}

func TestReadyWithMixedDependencies(t *testing.T) {
	t.Parallel()

	svc := NewService([]DependencyPinger{
		fakePinger{name: "mysql"},
		fakePinger{name: "redis", err: errors.New("dial tcp: refused")},
	})

	got, ok := svc.Ready(context.Background())
	if ok {
		t.Fatalf("expected ready=false")
	}
	if got.Status != "not_ready" {
		t.Fatalf("expected status not_ready, got %s", got.Status)
	}
	if len(got.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(got.Dependencies))
	}
	if !got.Dependencies[0].Healthy {
		t.Fatalf("expected mysql healthy")
	}
	if got.Dependencies[1].Healthy || got.Dependencies[1].Error == "" {
		t.Fatalf("expected redis unhealthy with error")
	}
}
