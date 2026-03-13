package store

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

func ConnectPostgres(_ context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}

func WaitForPostgres(ctx context.Context, db *sql.DB, attempts int, delay time.Duration) error {
	var lastErr error

	for i := 0; i < attempts; i++ {
		if err := db.PingContext(ctx); err == nil {
			return nil
		} else {
			lastErr = err
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
