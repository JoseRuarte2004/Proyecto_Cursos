package mq

import (
	"context"
	"errors"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ConnectionManager struct {
	url  string
	mu   sync.RWMutex
	conn *amqp.Connection
}

func NewConnectionManager(url string) *ConnectionManager {
	return &ConnectionManager{url: url}
}

func (m *ConnectionManager) Connect(ctx context.Context, attempts int, delay time.Duration) error {
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := m.ensureConnected(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if i == attempts-1 || delay <= 0 {
			break
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return lastErr
}

func (m *ConnectionManager) Channel(ctx context.Context) (*amqp.Channel, error) {
	if err := m.Connect(ctx, 1, 0); err != nil {
		return nil, err
	}

	m.mu.RLock()
	conn := m.conn
	m.mu.RUnlock()
	if conn == nil || conn.IsClosed() {
		return nil, errors.New("rabbitmq connection is not available")
	}

	return conn.Channel()
}

func (m *ConnectionManager) Ping(ctx context.Context) error {
	channel, err := m.Channel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	return nil
}

func (m *ConnectionManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil || m.conn.IsClosed() {
		return nil
	}

	return m.conn.Close()
}

func (m *ConnectionManager) ensureConnected() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != nil && !m.conn.IsClosed() {
		return nil
	}

	conn, err := amqp.Dial(m.url)
	if err != nil {
		return err
	}

	m.conn = conn
	return nil
}
