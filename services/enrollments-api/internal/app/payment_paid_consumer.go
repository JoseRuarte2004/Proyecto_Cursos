package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/mq"
	"proyecto-cursos/internal/platform/requestid"
	"proyecto-cursos/services/enrollments-api/internal/domain"
	"proyecto-cursos/services/enrollments-api/internal/service"
)

type paymentConfirmService interface {
	ConfirmPaidEnrollment(ctx context.Context, userID, courseID string) (*domain.Enrollment, error)
}

type PaymentPaidConsumer struct {
	log        *logger.Logger
	manager    *mq.ConnectionManager
	service    paymentConfirmService
	issueStore paymentActivationIssueStore
	now        func() time.Time
}

type paymentActivationIssueStore interface {
	UpsertPaymentActivationIssue(ctx context.Context, issue domain.PaymentActivationIssue) error
	ResolvePaymentActivationIssue(ctx context.Context, orderID string, resolvedAt time.Time) error
}

type paymentPaidEvent struct {
	OrderID     string `json:"orderId"`
	UserID      string `json:"userId"`
	CourseID    string `json:"courseId"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
	Provider    string `json:"provider"`
}

func NewPaymentPaidConsumer(log *logger.Logger, manager *mq.ConnectionManager, svc paymentConfirmService, issueStore paymentActivationIssueStore) *PaymentPaidConsumer {
	return &PaymentPaidConsumer{
		log:        log,
		manager:    manager,
		service:    svc,
		issueStore: issueStore,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (c *PaymentPaidConsumer) Start(ctx context.Context) {
	go func() {
		for {
			if err := c.consume(ctx); err != nil && !errors.Is(err, context.Canceled) {
				c.log.Error(context.Background(), "payment consumer stopped", map[string]any{
					"error": err.Error(),
				})
			}

			if ctx.Err() != nil {
				return
			}

			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func (c *PaymentPaidConsumer) consume(ctx context.Context) error {
	channel, err := c.manager.Channel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	if err := channel.ExchangeDeclare("payments", "topic", true, false, false, false, nil); err != nil {
		return err
	}

	queue, err := channel.QueueDeclare("enrollments.payment.paid", true, false, false, false, nil)
	if err != nil {
		return err
	}

	if err := channel.QueueBind(queue.Name, "payment.paid", "payments", false, nil); err != nil {
		return err
	}

	deliveries, err := channel.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	closeCh := channel.NotifyClose(make(chan *amqp.Error, 1))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-closeCh:
			if err == nil {
				return nil
			}
			return err
		case delivery, ok := <-deliveries:
			if !ok {
				return nil
			}
			c.handleDelivery(delivery)
		}
	}
}

func (c *PaymentPaidConsumer) handleDelivery(delivery amqp.Delivery) {
	eventID := delivery.MessageId
	if eventID == "" {
		eventID = uuid.NewString()
	}

	requestID := headerString(delivery.Headers, "requestId")
	if requestID == "" {
		requestID = eventID
	}
	ctx := requestid.WithContext(context.Background(), requestID)

	var event paymentPaidEvent
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		c.log.Error(ctx, "invalid payment event payload", map[string]any{
			"eventId": eventID,
			"error":   err.Error(),
		})
		_ = delivery.Ack(false)
		return
	}

	if _, err := c.service.ConfirmPaidEnrollment(ctx, event.UserID, event.CourseID); err != nil {
		if errors.Is(err, service.ErrPendingEnrollmentMissing) {
			if storeErr := c.recordActivationIssue(ctx, event, domain.PaymentActivationIssueManualReview, "pending_enrollment_missing", err.Error(), nil); storeErr != nil {
				c.log.Error(ctx, "failed to persist payment activation issue", map[string]any{
					"eventId":  eventID,
					"orderId":  event.OrderID,
					"userId":   event.UserID,
					"courseId": event.CourseID,
					"error":    storeErr.Error(),
				})
				_ = delivery.Nack(false, true)
				return
			}
			c.log.Error(ctx, "payment event could not confirm enrollment", map[string]any{
				"eventId":  eventID,
				"orderId":  event.OrderID,
				"userId":   event.UserID,
				"courseId": event.CourseID,
				"error":    err.Error(),
				"status":   string(domain.PaymentActivationIssueManualReview),
			})
			_ = delivery.Ack(false)
			return
		}
		if errors.Is(err, service.ErrCourseFull) {
			nextAttemptAt := c.now().Add(paymentActivationRetryDelay(1))
			if storeErr := c.recordActivationIssue(ctx, event, domain.PaymentActivationIssueRetryable, "course_full", err.Error(), &nextAttemptAt); storeErr != nil {
				c.log.Error(ctx, "failed to persist payment activation issue", map[string]any{
					"eventId":  eventID,
					"orderId":  event.OrderID,
					"userId":   event.UserID,
					"courseId": event.CourseID,
					"error":    storeErr.Error(),
				})
				_ = delivery.Nack(false, true)
				return
			}
			c.log.Error(ctx, "payment event deferred for retry", map[string]any{
				"eventId":       eventID,
				"orderId":       event.OrderID,
				"userId":        event.UserID,
				"courseId":      event.CourseID,
				"error":         err.Error(),
				"status":        string(domain.PaymentActivationIssueRetryable),
				"nextAttemptAt": nextAttemptAt.Format(time.RFC3339),
			})
			_ = delivery.Ack(false)
			return
		}

		c.log.Error(ctx, "payment event processing failed", map[string]any{
			"eventId":  eventID,
			"orderId":  event.OrderID,
			"userId":   event.UserID,
			"courseId": event.CourseID,
			"error":    err.Error(),
		})
		_ = delivery.Nack(false, true)
		return
	}

	if c.issueStore != nil {
		if err := c.issueStore.ResolvePaymentActivationIssue(ctx, event.OrderID, c.now()); err != nil {
			c.log.Error(ctx, "failed to resolve payment activation issue", map[string]any{
				"eventId":  eventID,
				"orderId":  event.OrderID,
				"userId":   event.UserID,
				"courseId": event.CourseID,
				"error":    err.Error(),
			})
		}
	}

	c.log.Info(ctx, "payment event processed", map[string]any{
		"eventId":  eventID,
		"orderId":  event.OrderID,
		"userId":   event.UserID,
		"courseId": event.CourseID,
	})
	_ = delivery.Ack(false)
}

func headerString(headers amqp.Table, key string) string {
	if headers == nil {
		return ""
	}

	value, ok := headers[key]
	if !ok || value == nil {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func (c *PaymentPaidConsumer) recordActivationIssue(ctx context.Context, event paymentPaidEvent, status domain.PaymentActivationIssueStatus, reasonCode, lastError string, nextAttemptAt *time.Time) error {
	if c.issueStore == nil {
		return errors.New("payment activation issue store is not configured")
	}

	now := c.now()
	issue := domain.PaymentActivationIssue{
		ID:            uuid.NewString(),
		OrderID:       strings.TrimSpace(event.OrderID),
		UserID:        strings.TrimSpace(event.UserID),
		CourseID:      strings.TrimSpace(event.CourseID),
		ReasonCode:    strings.TrimSpace(reasonCode),
		Status:        status,
		LastError:     strings.TrimSpace(lastError),
		Attempts:      1,
		NextAttemptAt: nextAttemptAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	return c.issueStore.UpsertPaymentActivationIssue(ctx, issue)
}

func paymentActivationRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	return time.Duration(1<<uint(attempt-1)) * 30 * time.Second
}
