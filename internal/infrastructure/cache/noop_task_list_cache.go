package cache

import (
	"context"
	"time"
)

type NoopTaskListCache struct{}

func NewNoopTaskListCache() *NoopTaskListCache {
	return &NoopTaskListCache{}
}

func (c *NoopTaskListCache) Get(_ context.Context, _ string) ([]byte, bool, error) {
	return nil, false, nil
}

func (c *NoopTaskListCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}

func (c *NoopTaskListCache) InvalidateTeam(_ context.Context, _ int64) error {
	return nil
}
