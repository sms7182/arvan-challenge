package pkg

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type CacheService struct {
	Rdb *redis.Client
}

func (r CacheService) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	return r.Rdb.Set(ctx, key, value, expiration).Err()
}

func (r CacheService) Get(ctx context.Context, key string) (string, error) {
	return r.Rdb.Get(ctx, key).Result()
}

func (r CacheService) Delete(ctx context.Context, key string) error {
	return r.Rdb.Del(ctx, key).Err()
}
