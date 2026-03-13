package repository

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisVerificationCodeStore struct {
	client    *redis.Client
	keyPrefix string
}

func NewRedisVerificationCodeStore(client *redis.Client) *RedisVerificationCodeStore {
	return NewRedisVerificationCodeStoreWithPrefix(client, "verify_code:")
}

func NewRedisVerificationCodeStoreWithPrefix(client *redis.Client, keyPrefix string) *RedisVerificationCodeStore {
	trimmedPrefix := strings.TrimSpace(keyPrefix)
	if trimmedPrefix == "" {
		trimmedPrefix = "verify_code:"
	}

	return &RedisVerificationCodeStore{
		client:    client,
		keyPrefix: trimmedPrefix,
	}
}

func (s *RedisVerificationCodeStore) Set(ctx context.Context, email, code string, ttl time.Duration) error {
	return s.client.Set(ctx, s.verificationCodeKey(email), code, ttl).Err()
}

func (s *RedisVerificationCodeStore) Get(ctx context.Context, email string) (string, bool, error) {
	code, err := s.client.Get(ctx, s.verificationCodeKey(email)).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	return code, true, nil
}

func (s *RedisVerificationCodeStore) Delete(ctx context.Context, email string) error {
	return s.client.Del(ctx, s.verificationCodeKey(email)).Err()
}

func (s *RedisVerificationCodeStore) verificationCodeKey(email string) string {
	return s.keyPrefix + strings.ToLower(strings.TrimSpace(email))
}
