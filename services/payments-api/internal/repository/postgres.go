package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/lib/pq"

	"proyecto-cursos/services/payments-api/internal/domain"
	"proyecto-cursos/services/payments-api/internal/service"
)

type PostgresOrderRepository struct {
	db *sql.DB
}

type scanner interface {
	Scan(dest ...any) error
}

func NewPostgresOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db}
}

func (r *PostgresOrderRepository) Create(ctx context.Context, order domain.Order) (*domain.Order, error) {
	const query = `
		INSERT INTO orders (
			id,
			user_id,
			course_id,
			amount_cents,
			amount,
			currency,
			provider,
			provider_payment_id,
			provider_preference_id,
			external_reference,
			checkout_url,
			provider_status,
			status,
			idempotency_key,
			paid_at,
			failed_at,
			last_webhook_at,
			expires_at,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id, user_id, course_id, amount_cents, currency, provider, provider_payment_id, provider_preference_id, external_reference, checkout_url, provider_status, status, idempotency_key, paid_at, failed_at, last_webhook_at, expires_at, created_at, updated_at
	`

	created, err := scanOrder(r.db.QueryRowContext(
		ctx,
		query,
		order.ID,
		order.UserID,
		order.CourseID,
		order.AmountCents,
		service.CentsToNumericText(order.AmountCents),
		order.Currency,
		order.Provider,
		order.ProviderPaymentID,
		order.ProviderPreferenceID,
		order.ExternalReference,
		order.CheckoutURL,
		order.ProviderStatus,
		order.Status,
		order.IdempotencyKey,
		order.PaidAt,
		order.FailedAt,
		order.LastWebhookAt,
		order.ExpiresAt,
		order.CreatedAt,
		order.UpdatedAt,
	))
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			if pqErr.Constraint == "idx_orders_user_course_open" {
				return nil, service.ErrOpenOrderAlreadyExists
			}
			if pqErr.Constraint == "idx_orders_scoped_idempotency" || pqErr.Constraint == "orders_idempotency_key_key" {
				return nil, service.ErrDuplicateIdempotencyKey
			}
			return nil, service.ErrDuplicateIdempotencyKey
		}
		return nil, err
	}

	return created, nil
}

func (r *PostgresOrderRepository) GetByID(ctx context.Context, orderID string) (*domain.Order, error) {
	order, err := scanOrder(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, course_id, amount_cents, currency, provider, provider_payment_id, provider_preference_id, external_reference, checkout_url, provider_status, status, idempotency_key, paid_at, failed_at, last_webhook_at, expires_at, created_at, updated_at
		FROM orders
		WHERE id = $1
	`, orderID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrOrderNotFound
		}
		return nil, err
	}

	return order, nil
}

func (r *PostgresOrderRepository) GetByIdempotencyKey(ctx context.Context, userID, courseID string, provider domain.Provider, idempotencyKey string) (*domain.Order, error) {
	order, err := scanOrder(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, course_id, amount_cents, currency, provider, provider_payment_id, provider_preference_id, external_reference, checkout_url, provider_status, status, idempotency_key, paid_at, failed_at, last_webhook_at, expires_at, created_at, updated_at
		FROM orders
		WHERE user_id = $1
		  AND course_id = $2
		  AND provider = $3
		  AND idempotency_key = $4
	`, userID, courseID, provider, idempotencyKey))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrOrderNotFound
		}
		return nil, err
	}

	return order, nil
}

func (r *PostgresOrderRepository) GetOpenByUserCourse(ctx context.Context, userID, courseID string) (*domain.Order, error) {
	order, err := scanOrder(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, course_id, amount_cents, currency, provider, provider_payment_id, provider_preference_id, external_reference, checkout_url, provider_status, status, idempotency_key, paid_at, failed_at, last_webhook_at, expires_at, created_at, updated_at
		FROM orders
		WHERE user_id = $1
		  AND course_id = $2
		  AND status IN ('created', 'pending')
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, courseID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrOrderNotFound
		}
		return nil, err
	}

	return order, nil
}

func (r *PostgresOrderRepository) UpdateCheckout(ctx context.Context, orderID string, checkout service.UpdateCheckoutInput) (*domain.Order, error) {
	order, err := scanOrder(r.db.QueryRowContext(ctx, `
		UPDATE orders
		SET checkout_url = $2,
			external_reference = $3,
			provider_preference_id = $4,
			provider_status = COALESCE($5, provider_status),
			updated_at = $6
		WHERE id = $1
		RETURNING id, user_id, course_id, amount_cents, currency, provider, provider_payment_id, provider_preference_id, external_reference, checkout_url, provider_status, status, idempotency_key, paid_at, failed_at, last_webhook_at, expires_at, created_at, updated_at
	`, orderID, checkout.CheckoutURL, checkout.ExternalReference, checkout.ProviderReferenceID, checkout.ProviderStatus, checkout.UpdatedAt))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrOrderNotFound
		}
		return nil, err
	}

	return order, nil
}

func (r *PostgresOrderRepository) CreateWebhookEvent(ctx context.Context, event domain.PaymentWebhookEvent) (*domain.PaymentWebhookEvent, bool, error) {
	created, err := scanWebhookEvent(r.db.QueryRowContext(ctx, `
		INSERT INTO payment_webhook_events (
			id,
			provider,
			event_key,
			request_id,
			topic,
			action,
			resource_id,
			order_id,
			payload,
			created_at,
			processed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (provider, event_key) DO NOTHING
		RETURNING id, provider, event_key, request_id, topic, action, resource_id, order_id, payload, created_at, processed_at
	`, event.ID, event.Provider, event.EventKey, event.RequestID, event.Topic, event.Action, event.ResourceID, event.OrderID, event.Payload, event.CreatedAt, event.ProcessedAt))
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	existing, err := scanWebhookEvent(r.db.QueryRowContext(ctx, `
		SELECT id, provider, event_key, request_id, topic, action, resource_id, order_id, payload, created_at, processed_at
		FROM payment_webhook_events
		WHERE provider = $1 AND event_key = $2
	`, event.Provider, event.EventKey))
	if err != nil {
		return nil, false, err
	}

	return existing, false, nil
}

func (r *PostgresOrderRepository) EnqueueWebhookJob(ctx context.Context, job domain.PaymentWebhookJob) (*domain.PaymentWebhookJob, bool, error) {
	created, err := scanWebhookJob(r.db.QueryRowContext(ctx, `
		INSERT INTO payment_webhook_jobs (
			id,
			provider,
			dedupe_key,
			resource_id,
			request_id,
			signature_ts,
			topic,
			action,
			payload,
			attempts,
			received_at,
			available_at,
			processed_at,
			locked_at,
			locked_by,
			last_error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULL, NULL, NULL, NULL)
		ON CONFLICT (provider, dedupe_key) DO NOTHING
		RETURNING id, provider, dedupe_key, resource_id, request_id, signature_ts, topic, action, payload, attempts, received_at, available_at, processed_at, locked_at, locked_by, last_error
	`, job.ID, job.Provider, job.DedupeKey, job.ResourceID, job.RequestID, job.SignatureTimestamp, job.Topic, job.Action, job.Payload, job.Attempts, job.ReceivedAt, job.AvailableAt))
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	existing, err := scanWebhookJob(r.db.QueryRowContext(ctx, `
		SELECT id, provider, dedupe_key, resource_id, request_id, signature_ts, topic, action, payload, attempts, received_at, available_at, processed_at, locked_at, locked_by, last_error
		FROM payment_webhook_jobs
		WHERE provider = $1 AND dedupe_key = $2
	`, job.Provider, job.DedupeKey))
	if err != nil {
		return nil, false, err
	}

	return existing, false, nil
}

func (r *PostgresOrderRepository) ClaimWebhookJobByID(ctx context.Context, jobID, workerID string, now time.Time) (*domain.PaymentWebhookJob, error) {
	job, err := scanWebhookJob(r.db.QueryRowContext(ctx, `
		UPDATE payment_webhook_jobs
		SET locked_at = $2,
			locked_by = $3
		WHERE id = $1
		  AND processed_at IS NULL
		  AND available_at <= $2
		  AND (locked_at IS NULL OR locked_at <= $4)
		RETURNING id, provider, dedupe_key, resource_id, request_id, signature_ts, topic, action, payload, attempts, received_at, available_at, processed_at, locked_at, locked_by, last_error
	`, jobID, now, workerID, now.Add(-2*time.Minute)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrOrderNotFound
		}
		return nil, err
	}

	return job, nil
}

func (r *PostgresOrderRepository) ClaimWebhookJobs(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentWebhookJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(ctx, `
		WITH candidate AS (
			SELECT id
			FROM payment_webhook_jobs
			WHERE processed_at IS NULL
			  AND available_at <= $1
			  AND (locked_at IS NULL OR locked_at <= $2)
			ORDER BY available_at ASC, received_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $4
		)
		UPDATE payment_webhook_jobs jobs
		SET locked_at = $1,
			locked_by = $3
		FROM candidate
		WHERE jobs.id = candidate.id
		RETURNING jobs.id, jobs.provider, jobs.dedupe_key, jobs.resource_id, jobs.request_id, jobs.signature_ts, jobs.topic, jobs.action, jobs.payload, jobs.attempts, jobs.received_at, jobs.available_at, jobs.processed_at, jobs.locked_at, jobs.locked_by, jobs.last_error
	`, now, now.Add(-2*time.Minute), workerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs, err := collectWebhookJobs(rows)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (r *PostgresOrderRepository) MarkWebhookJobProcessed(ctx context.Context, jobID string, processedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE payment_webhook_jobs
		SET processed_at = $2,
			locked_at = NULL,
			locked_by = NULL,
			last_error = NULL
		WHERE id = $1
	`, jobID, processedAt)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrOrderNotFound
	}

	return nil
}

func (r *PostgresOrderRepository) ReleaseWebhookJob(ctx context.Context, jobID string, availableAt time.Time, lastError string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE payment_webhook_jobs
		SET attempts = attempts + 1,
			available_at = $2,
			locked_at = NULL,
			locked_by = NULL,
			last_error = $3
		WHERE id = $1
	`, jobID, availableAt, strings.TrimSpace(lastError))
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrOrderNotFound
	}

	return nil
}

func (r *PostgresOrderRepository) UpsertOrderIssue(ctx context.Context, issue domain.PaymentOrderIssue) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO payment_order_issues (
			id,
			order_id,
			issue_key,
			issue_type,
			provider_payment_id,
			details,
			status,
			created_at,
			updated_at,
			resolved_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (issue_key) DO UPDATE
		SET details = EXCLUDED.details,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at,
			resolved_at = EXCLUDED.resolved_at
	`, issue.ID, issue.OrderID, issue.IssueKey, issue.IssueType, issue.ProviderPaymentID, issue.Details, issue.Status, issue.CreatedAt, issue.UpdatedAt, issue.ResolvedAt)
	return err
}

func (r *PostgresOrderRepository) ApplyWebhookResult(ctx context.Context, input service.ApplyWebhookResultInput) (*domain.Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	updated, err := scanOrder(tx.QueryRowContext(ctx, `
		UPDATE orders
		SET status = $2,
			provider_payment_id = CASE
				WHEN provider_payment_id IS NULL OR provider_payment_id = '' THEN COALESCE($3, provider_payment_id)
				ELSE provider_payment_id
			END,
			provider_status = COALESCE($4, provider_status),
			last_webhook_at = COALESCE($5, last_webhook_at),
			paid_at = COALESCE($6, paid_at),
			failed_at = COALESCE($7, failed_at),
			expires_at = $8,
			updated_at = $9
		WHERE id = $1
		RETURNING id, user_id, course_id, amount_cents, currency, provider, provider_payment_id, provider_preference_id, external_reference, checkout_url, provider_status, status, idempotency_key, paid_at, failed_at, last_webhook_at, expires_at, created_at, updated_at
	`,
		input.Update.OrderID,
		input.Update.Status,
		input.Update.ProviderPaymentID,
		input.Update.ProviderStatus,
		input.Update.LastWebhookAt,
		input.Update.PaidAt,
		input.Update.FailedAt,
		input.Update.ExpiresAt,
		input.Update.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrOrderNotFound
		}
		return nil, err
	}

	if input.OutboxEvent != nil {
		if err = insertOrUpdateOutboxEvent(ctx, tx, *input.OutboxEvent); err != nil {
			return nil, err
		}
	}

	var orderID any
	if strings.TrimSpace(input.EventOrderID) != "" {
		orderID = input.EventOrderID
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE payment_webhook_events
		SET order_id = COALESCE($3, order_id),
			processed_at = $4
		WHERE provider = $1 AND event_key = $2
	`, input.EventProvider, input.EventKey, orderID, input.ProcessedAt)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, service.ErrWebhookEventNotFound
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return updated, nil
}

func (r *PostgresOrderRepository) EnsureOutboxEvent(ctx context.Context, event domain.PaymentOutboxEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO payment_outbox_events (
			id,
			event_type,
			order_id,
			payload,
			attempts,
			created_at,
			available_at,
			published_at,
			locked_at,
			locked_by,
			last_error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, NULL, NULL)
		ON CONFLICT (event_type, order_id) DO UPDATE
		SET payload = EXCLUDED.payload,
			available_at = LEAST(payment_outbox_events.available_at, EXCLUDED.available_at),
			last_error = NULL
		WHERE payment_outbox_events.published_at IS NULL
	`, event.ID, event.EventType, event.OrderID, event.Payload, event.Attempts, event.CreatedAt, event.AvailableAt)
	return err
}

func (r *PostgresOrderRepository) ClaimOutboxEvents(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentOutboxEvent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(ctx, `
		WITH candidate AS (
			SELECT id
			FROM payment_outbox_events
			WHERE published_at IS NULL
			  AND available_at <= $1
			  AND (locked_at IS NULL OR locked_at <= $2)
			ORDER BY available_at ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $4
		)
		UPDATE payment_outbox_events events
		SET locked_at = $1,
			locked_by = $3
		FROM candidate
		WHERE events.id = candidate.id
		RETURNING events.id, events.event_type, events.order_id, events.payload, events.attempts, events.created_at, events.available_at, events.published_at, events.locked_at, events.locked_by, events.last_error
	`, now, now.Add(-2*time.Minute), workerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events, err := collectOutboxEvents(rows)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return events, nil
}

func (r *PostgresOrderRepository) MarkOutboxEventPublished(ctx context.Context, eventID string, publishedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE payment_outbox_events
		SET published_at = $2,
			locked_at = NULL,
			locked_by = NULL,
			last_error = NULL
		WHERE id = $1
	`, eventID, publishedAt)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrOrderNotFound
	}

	return nil
}

func (r *PostgresOrderRepository) ReleaseOutboxEvent(ctx context.Context, eventID string, availableAt time.Time, lastError string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE payment_outbox_events
		SET attempts = attempts + 1,
			available_at = $2,
			locked_at = NULL,
			locked_by = NULL,
			last_error = $3
		WHERE id = $1
	`, eventID, availableAt, strings.TrimSpace(lastError))
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrOrderNotFound
	}

	return nil
}

func (r *PostgresOrderRepository) ListOpenOrdersForReconciliation(ctx context.Context, provider domain.Provider, now, olderThan time.Time, limit int) ([]domain.Order, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, course_id, amount_cents, currency, provider, provider_payment_id, provider_preference_id, external_reference, checkout_url, provider_status, status, idempotency_key, paid_at, failed_at, last_webhook_at, expires_at, created_at, updated_at
		FROM orders
		WHERE provider = $1
		  AND status IN ('created', 'pending')
		  AND external_reference IS NOT NULL
		  AND (expires_at IS NULL OR expires_at > $2)
		  AND COALESCE(last_webhook_at, updated_at, created_at) <= $3
		ORDER BY COALESCE(last_webhook_at, updated_at, created_at) ASC
		LIMIT $4
	`, provider, now, olderThan, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectOrders(rows)
}

func (r *PostgresOrderRepository) ListExpiredOpenOrders(ctx context.Context, now time.Time, limit int) ([]domain.Order, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, course_id, amount_cents, currency, provider, provider_payment_id, provider_preference_id, external_reference, checkout_url, provider_status, status, idempotency_key, paid_at, failed_at, last_webhook_at, expires_at, created_at, updated_at
		FROM orders
		WHERE status IN ('created', 'pending')
		  AND expires_at IS NOT NULL
		  AND expires_at <= $1
		ORDER BY expires_at ASC
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectOrders(rows)
}

func (r *PostgresOrderRepository) ExpireOrder(ctx context.Context, orderID string, providerStatus string, failedAt time.Time) (*domain.Order, error) {
	order, err := scanOrder(r.db.QueryRowContext(ctx, `
		UPDATE orders
		SET status = 'failed',
			provider_status = CASE
				WHEN provider_status IS NULL OR provider_status = '' OR provider_status IN ('created', 'pending')
					THEN $2
				ELSE provider_status
			END,
			failed_at = COALESCE(failed_at, $3),
			expires_at = NULL,
			updated_at = $3
		WHERE id = $1
		  AND status IN ('created', 'pending')
		RETURNING id, user_id, course_id, amount_cents, currency, provider, provider_payment_id, provider_preference_id, external_reference, checkout_url, provider_status, status, idempotency_key, paid_at, failed_at, last_webhook_at, expires_at, created_at, updated_at
	`, orderID, strings.TrimSpace(providerStatus), failedAt))
	if err == nil {
		return order, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return r.GetByID(ctx, orderID)
}

func (r *PostgresOrderRepository) HasOtherOpenOrder(ctx context.Context, userID, courseID, excludeOrderID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM orders
			WHERE user_id = $1
			  AND course_id = $2
			  AND id <> $3
			  AND status IN ('created', 'pending')
		)
	`, userID, courseID, excludeOrderID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func insertOrUpdateOutboxEvent(ctx context.Context, tx *sql.Tx, event domain.PaymentOutboxEvent) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO payment_outbox_events (
			id,
			event_type,
			order_id,
			payload,
			attempts,
			created_at,
			available_at,
			published_at,
			locked_at,
			locked_by,
			last_error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, NULL, NULL)
		ON CONFLICT (event_type, order_id) DO UPDATE
		SET payload = EXCLUDED.payload,
			available_at = LEAST(payment_outbox_events.available_at, EXCLUDED.available_at),
			last_error = NULL
		WHERE payment_outbox_events.published_at IS NULL
	`, event.ID, event.EventType, event.OrderID, event.Payload, event.Attempts, event.CreatedAt, event.AvailableAt)
	return err
}

func collectOrders(rows *sql.Rows) ([]domain.Order, error) {
	orders := make([]domain.Order, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func collectOutboxEvents(rows *sql.Rows) ([]domain.PaymentOutboxEvent, error) {
	events := make([]domain.PaymentOutboxEvent, 0)
	for rows.Next() {
		event, err := scanOutboxEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func collectWebhookJobs(rows *sql.Rows) ([]domain.PaymentWebhookJob, error) {
	jobs := make([]domain.PaymentWebhookJob, 0)
	for rows.Next() {
		job, err := scanWebhookJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func scanOrder(row scanner) (*domain.Order, error) {
	var (
		order                domain.Order
		providerPaymentID    sql.NullString
		providerPreferenceID sql.NullString
		externalReference    sql.NullString
		checkoutURL          sql.NullString
		providerStatus       sql.NullString
		paidAt               sql.NullTime
		failedAt             sql.NullTime
		lastWebhookAt        sql.NullTime
		expiresAt            sql.NullTime
	)

	if err := row.Scan(
		&order.ID,
		&order.UserID,
		&order.CourseID,
		&order.AmountCents,
		&order.Currency,
		&order.Provider,
		&providerPaymentID,
		&providerPreferenceID,
		&externalReference,
		&checkoutURL,
		&providerStatus,
		&order.Status,
		&order.IdempotencyKey,
		&paidAt,
		&failedAt,
		&lastWebhookAt,
		&expiresAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	); err != nil {
		return nil, err
	}

	order.ProviderPaymentID = nullStringPointer(providerPaymentID)
	order.ProviderPreferenceID = nullStringPointer(providerPreferenceID)
	order.ExternalReference = nullStringPointer(externalReference)
	order.CheckoutURL = nullStringPointer(checkoutURL)
	order.ProviderStatus = nullStringPointer(providerStatus)
	order.PaidAt = nullTimePointer(paidAt)
	order.FailedAt = nullTimePointer(failedAt)
	order.LastWebhookAt = nullTimePointer(lastWebhookAt)
	order.ExpiresAt = nullTimePointer(expiresAt)

	return &order, nil
}

func scanWebhookEvent(row scanner) (*domain.PaymentWebhookEvent, error) {
	var (
		event       domain.PaymentWebhookEvent
		requestID   sql.NullString
		topic       sql.NullString
		action      sql.NullString
		resourceID  sql.NullString
		orderID     sql.NullString
		processedAt sql.NullTime
	)

	if err := row.Scan(
		&event.ID,
		&event.Provider,
		&event.EventKey,
		&requestID,
		&topic,
		&action,
		&resourceID,
		&orderID,
		&event.Payload,
		&event.CreatedAt,
		&processedAt,
	); err != nil {
		return nil, err
	}

	event.RequestID = nullStringPointer(requestID)
	event.Topic = nullStringPointer(topic)
	event.Action = nullStringPointer(action)
	event.ResourceID = nullStringPointer(resourceID)
	event.OrderID = nullStringPointer(orderID)
	event.ProcessedAt = nullTimePointer(processedAt)

	return &event, nil
}

func scanOutboxEvent(row scanner) (*domain.PaymentOutboxEvent, error) {
	var (
		event       domain.PaymentOutboxEvent
		publishedAt sql.NullTime
		lockedAt    sql.NullTime
		lockedBy    sql.NullString
		lastError   sql.NullString
	)

	if err := row.Scan(
		&event.ID,
		&event.EventType,
		&event.OrderID,
		&event.Payload,
		&event.Attempts,
		&event.CreatedAt,
		&event.AvailableAt,
		&publishedAt,
		&lockedAt,
		&lockedBy,
		&lastError,
	); err != nil {
		return nil, err
	}

	event.PublishedAt = nullTimePointer(publishedAt)
	event.LockedAt = nullTimePointer(lockedAt)
	event.LockedBy = nullStringPointer(lockedBy)
	event.LastError = nullStringPointer(lastError)

	return &event, nil
}

func scanWebhookJob(row scanner) (*domain.PaymentWebhookJob, error) {
	var (
		job                domain.PaymentWebhookJob
		requestID          sql.NullString
		signatureTimestamp sql.NullString
		topic              sql.NullString
		action             sql.NullString
		processedAt        sql.NullTime
		lockedAt           sql.NullTime
		lockedBy           sql.NullString
		lastError          sql.NullString
	)

	if err := row.Scan(
		&job.ID,
		&job.Provider,
		&job.DedupeKey,
		&job.ResourceID,
		&requestID,
		&signatureTimestamp,
		&topic,
		&action,
		&job.Payload,
		&job.Attempts,
		&job.ReceivedAt,
		&job.AvailableAt,
		&processedAt,
		&lockedAt,
		&lockedBy,
		&lastError,
	); err != nil {
		return nil, err
	}

	job.RequestID = nullStringPointer(requestID)
	job.SignatureTimestamp = nullStringPointer(signatureTimestamp)
	job.Topic = nullStringPointer(topic)
	job.Action = nullStringPointer(action)
	job.ProcessedAt = nullTimePointer(processedAt)
	job.LockedAt = nullTimePointer(lockedAt)
	job.LockedBy = nullStringPointer(lockedBy)
	job.LastError = nullStringPointer(lastError)

	return &job, nil
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}

	return &value.String
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}

	return &value.Time
}
