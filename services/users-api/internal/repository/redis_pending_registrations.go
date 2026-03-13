package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"proyecto-cursos/services/users-api/internal/service"
)

type RedisPendingRegistrationStore struct {
	client    *redis.Client
	keyPrefix string
}

func NewRedisPendingRegistrationStore(client *redis.Client) *RedisPendingRegistrationStore {
	return NewRedisPendingRegistrationStoreWithPrefix(client, "pending_registration:")
}

func NewRedisPendingRegistrationStoreWithPrefix(client *redis.Client, keyPrefix string) *RedisPendingRegistrationStore {
	trimmedPrefix := strings.TrimSpace(keyPrefix)
	if trimmedPrefix == "" {
		trimmedPrefix = "pending_registration:"
	}

	return &RedisPendingRegistrationStore{
		client:    client,
		keyPrefix: trimmedPrefix,
	}
}

func (s *RedisPendingRegistrationStore) Set(ctx context.Context, email string, pending service.PendingRegistration, ttl time.Duration) error {
	payload, err := json.Marshal(pending)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, s.pendingRegistrationKey(email), payload, ttl).Err()
}

func (s *RedisPendingRegistrationStore) Get(ctx context.Context, email string) (service.PendingRegistration, bool, error) {
	payload, err := s.client.Get(ctx, s.pendingRegistrationKey(email)).Result()
	if err == redis.Nil {
		return service.PendingRegistration{}, false, nil
	}
	if err != nil {
		return service.PendingRegistration{}, false, err
	}

	var pending service.PendingRegistration
	if err := json.Unmarshal([]byte(payload), &pending); err != nil {
		return service.PendingRegistration{}, false, err
	}

	return pending, true, nil
}

func (s *RedisPendingRegistrationStore) Delete(ctx context.Context, email string) error {
	return s.client.Del(ctx, s.pendingRegistrationKey(email)).Err()
}

func (s *RedisPendingRegistrationStore) pendingRegistrationKey(email string) string {
	return s.keyPrefix + strings.ToLower(strings.TrimSpace(email))
}
