package store

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func ConnectRedis(redisURL string) (*redis.Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	return redis.NewClient(options), nil
}

func PingRedis(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}
