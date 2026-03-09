package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const taskListKeyPrefix = "tasks:list:team"

type RedisTaskListCache struct {
	client *redis.Client
}

func NewRedisTaskListCache(client *redis.Client) *RedisTaskListCache {
	return &RedisTaskListCache{client: client}
}

func (c *RedisTaskListCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	return value, true, nil
}

func (c *RedisTaskListCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisTaskListCache) InvalidateTeam(ctx context.Context, teamID int64) error {
	pattern := fmt.Sprintf("%s:%d:*", taskListKeyPrefix, teamID)
	var cursor uint64

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}
