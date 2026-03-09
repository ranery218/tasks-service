package readiness

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisPinger struct {
	client *redis.Client
}

func NewRedisPinger(client *redis.Client) RedisPinger {
	return RedisPinger{client: client}
}

func (p RedisPinger) Name() string {
	return "redis"
}

func (p RedisPinger) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
