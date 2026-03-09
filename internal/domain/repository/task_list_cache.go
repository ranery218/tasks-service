package repository

import (
	"context"
	"time"
)

type TaskListCache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	InvalidateTeam(ctx context.Context, teamID int64) error
}
