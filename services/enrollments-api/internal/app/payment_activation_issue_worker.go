package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/services/enrollments-api/internal/domain"
	"proyecto-cursos/services/enrollments-api/internal/service"
)

type paymentActivationRetryStore interface {
	ClaimPaymentActivationIssues(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentActivationIssue, error)
	ReleasePaymentActivationIssue(ctx context.Context, issueID string, status domain.PaymentActivationIssueStatus, nextAttemptAt *time.Time, lastError string) error
	ResolvePaymentActivationIssue(ctx context.Context, orderID string, resolvedAt time.Time) error
}

type PaymentActivationIssueWorker struct {
	log       *logger.Logger
	service   paymentConfirmService
	store     paymentActivationRetryStore
	interval  time.Duration
	batchSize int
	now       func() time.Time
}

func NewPaymentActivationIssueWorker(log *logger.Logger, svc paymentConfirmService, store paymentActivationRetryStore, interval time.Duration, batchSize int) *PaymentActivationIssueWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 20
	}

	return &PaymentActivationIssueWorker{
		log:       log,
		service:   svc,
		store:     store,
		interval:  interval,
		batchSize: batchSize,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (w *PaymentActivationIssueWorker) Start(ctx context.Context) {
	if w.store == nil {
		return
	}

	go func() {
		workerID := uuid.NewString()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			if err := w.run(ctx, workerID); err != nil && ctx.Err() == nil {
				w.log.Error(context.Background(), "payment activation worker failed", map[string]any{
					"error": err.Error(),
				})
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (w *PaymentActivationIssueWorker) run(ctx context.Context, workerID string) error {
	issues, err := w.store.ClaimPaymentActivationIssues(ctx, workerID, w.now(), w.batchSize)
	if err != nil {
		return err
	}

	for _, issue := range issues {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if _, err := w.service.ConfirmPaidEnrollment(ctx, issue.UserID, issue.CourseID); err != nil {
			switch {
			case errors.Is(err, service.ErrCourseFull):
				nextAttemptAt := w.now().Add(paymentActivationRetryDelay(issue.Attempts + 1))
				nextStatus := domain.PaymentActivationIssueRetryable
				if issue.Attempts+1 >= 6 {
					nextStatus = domain.PaymentActivationIssueManualReview
					nextAttemptAt = time.Time{}
				}
				var nextAttemptPtr *time.Time
				if !nextAttemptAt.IsZero() {
					nextAttemptPtr = &nextAttemptAt
				}
				if releaseErr := w.store.ReleasePaymentActivationIssue(ctx, issue.ID, nextStatus, nextAttemptPtr, err.Error()); releaseErr != nil {
					return releaseErr
				}
			case errors.Is(err, service.ErrPendingEnrollmentMissing):
				if releaseErr := w.store.ReleasePaymentActivationIssue(ctx, issue.ID, domain.PaymentActivationIssueManualReview, nil, err.Error()); releaseErr != nil {
					return releaseErr
				}
			default:
				nextAttemptAt := w.now().Add(paymentActivationRetryDelay(issue.Attempts + 1))
				if releaseErr := w.store.ReleasePaymentActivationIssue(ctx, issue.ID, domain.PaymentActivationIssueRetryable, &nextAttemptAt, err.Error()); releaseErr != nil {
					return releaseErr
				}
			}
			continue
		}

		if err := w.store.ResolvePaymentActivationIssue(ctx, issue.OrderID, w.now()); err != nil {
			return err
		}
	}

	return nil
}
