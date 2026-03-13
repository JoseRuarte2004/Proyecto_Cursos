package app

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type CourseCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewCourseCache(client *redis.Client, ttl time.Duration) *CourseCache {
	return &CourseCache{
		client: client,
		ttl:    ttl,
	}
}

func (c *CourseCache) Get(ctx context.Context, key string) ([]byte, error) {
	payload, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *CourseCache) Set(ctx context.Context, key string, payload []byte) error {
	return c.client.Set(ctx, key, payload, c.ttl).Err()
}

func (c *CourseCache) InvalidatePublic(ctx context.Context) error {
	prefixes := []string{
		"courses:public:list:",
		"courses:public:detail:",
	}

	for _, prefix := range prefixes {
		var cursor uint64
		for {
			keys, nextCursor, err := c.client.Scan(ctx, cursor, prefix+"*", 100).Result()
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
	}

	return nil
}
