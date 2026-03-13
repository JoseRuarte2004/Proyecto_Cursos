package app

import (
	"context"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"

	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/services/enrollments-api/internal/domain"
	"proyecto-cursos/services/enrollments-api/internal/service"
)

type fakePaymentConfirmService struct {
	confirmFn func(ctx context.Context, userID, courseID string) (*domain.Enrollment, error)
}

func (f fakePaymentConfirmService) ConfirmPaidEnrollment(ctx context.Context, userID, courseID string) (*domain.Enrollment, error) {
	return f.confirmFn(ctx, userID, courseID)
}

type fakeAcknowledger struct {
	acked   bool
	nacked  bool
	requeue bool
}

func (f *fakeAcknowledger) Ack(uint64, bool) error {
	f.acked = true
	return nil
}

func (f *fakeAcknowledger) Nack(uint64, bool, bool) error {
	f.nacked = true
	return nil
}

func (f *fakeAcknowledger) Reject(uint64, bool) error {
	return nil
}

type fakePaymentActivationIssueStore struct{}

func (fakePaymentActivationIssueStore) UpsertPaymentActivationIssue(context.Context, domain.PaymentActivationIssue) error {
	return nil
}

func (fakePaymentActivationIssueStore) ResolvePaymentActivationIssue(context.Context, string, time.Time) error {
	return nil
}

type recordingPaymentActivationIssueStore struct {
	upserted []domain.PaymentActivationIssue
}

func (s *recordingPaymentActivationIssueStore) UpsertPaymentActivationIssue(_ context.Context, issue domain.PaymentActivationIssue) error {
	s.upserted = append(s.upserted, issue)
	return nil
}

func (s *recordingPaymentActivationIssueStore) ResolvePaymentActivationIssue(context.Context, string, time.Time) error {
	return nil
}

func TestPaymentPaidConsumerHandlesDelivery(t *testing.T) {
	t.Parallel()

	called := false
	consumer := NewPaymentPaidConsumer(logger.New("enrollments-api"), nil, fakePaymentConfirmService{
		confirmFn: func(_ context.Context, userID, courseID string) (*domain.Enrollment, error) {
			called = true
			require.Equal(t, "student-1", userID)
			require.Equal(t, "course-1", courseID)
			return &domain.Enrollment{
				ID:        "enr-1",
				UserID:    userID,
				CourseID:  courseID,
				Status:    domain.StatusActive,
				CreatedAt: time.Now().UTC(),
			}, nil
		},
	}, fakePaymentActivationIssueStore{})
	ack := &fakeAcknowledger{}

	consumer.handleDelivery(amqp.Delivery{
		Acknowledger: ack,
		MessageId:    "evt-1",
		Headers:      amqp.Table{"requestId": "req-1"},
		Body:         []byte(`{"orderId":"order-1","userId":"student-1","courseId":"course-1","amount":10,"currency":"USD","provider":"stripe"}`),
	})

	require.True(t, called)
	require.True(t, ack.acked)
	require.False(t, ack.nacked)
}

func TestPaymentPaidConsumerStoresManualReviewIssueForMissingPending(t *testing.T) {
	t.Parallel()

	store := &recordingPaymentActivationIssueStore{}
	consumer := NewPaymentPaidConsumer(logger.New("enrollments-api"), nil, fakePaymentConfirmService{
		confirmFn: func(context.Context, string, string) (*domain.Enrollment, error) {
			return nil, service.ErrPendingEnrollmentMissing
		},
	}, store)
	ack := &fakeAcknowledger{}

	consumer.handleDelivery(amqp.Delivery{
		Acknowledger: ack,
		MessageId:    "evt-2",
		Headers:      amqp.Table{"requestId": "req-2"},
		Body:         []byte(`{"orderId":"order-2","userId":"student-1","courseId":"course-1","amountCents":10,"currency":"USD","provider":"mercadopago"}`),
	})

	require.True(t, ack.acked)
	require.False(t, ack.nacked)
	require.Len(t, store.upserted, 1)
	require.Equal(t, domain.PaymentActivationIssueManualReview, store.upserted[0].Status)
	require.Equal(t, "pending_enrollment_missing", store.upserted[0].ReasonCode)
}
