package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store interface {
	IncrementAndGet(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) IncrementAndGet(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := s.client.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("redis pipeline exec: %w", err)
	}

	return incr.Val(), nil
}

type InMemoryStore struct {
	mu    sync.Mutex
	items map[string]memoryCounter
}

type memoryCounter struct {
	count     int64
	expiresAt time.Time
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{items: make(map[string]memoryCounter)}
}

func (s *InMemoryStore) IncrementAndGet(_ context.Context, key string, ttl time.Duration) (int64, error) {
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.items[key]
	if !exists || now.After(item.expiresAt) {
		item = memoryCounter{count: 0, expiresAt: now.Add(ttl)}
	}

	item.count++
	s.items[key] = item

	// Lazy cleanup for expired entries.
	for k, v := range s.items {
		if now.After(v.expiresAt) {
			delete(s.items, k)
		}
	}

	return item.count, nil
}
