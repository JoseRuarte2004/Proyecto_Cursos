package logger

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"proyecto-cursos/internal/platform/requestid"
)

type Logger struct {
	service string
	writer  io.Writer
	mu      sync.Mutex
}

func New(service string) *Logger {
	return &Logger{
		service: service,
		writer:  os.Stdout,
	}
}

func (l *Logger) Info(ctx context.Context, msg string, fields map[string]any) {
	l.log(ctx, "info", msg, fields)
}

func (l *Logger) Warn(ctx context.Context, msg string, fields map[string]any) {
	l.log(ctx, "warn", msg, fields)
}

func (l *Logger) Error(ctx context.Context, msg string, fields map[string]any) {
	l.log(ctx, "error", msg, fields)
}

func (l *Logger) log(ctx context.Context, level, msg string, fields map[string]any) {
	entry := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"service":   l.service,
		"level":     level,
		"requestId": requestid.FromContext(ctx),
		"msg":       msg,
	}

	for key, value := range fields {
		entry[key] = value
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.writer.Write(append(payload, '\n'))
}
