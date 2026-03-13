package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/services/payments-api/internal/domain"
)

type mockOrderRepository struct {
	createFn                   func(ctx context.Context, order domain.Order) (*domain.Order, error)
	getByIDFn                  func(ctx context.Context, orderID string) (*domain.Order, error)
	getByIdempotencyKeyFn      func(ctx context.Context, userID, courseID string, provider domain.Provider, idempotencyKey string) (*domain.Order, error)
	getOpenByUserCourseFn      func(ctx context.Context, userID, courseID string) (*domain.Order, error)
	updateCheckoutFn           func(ctx context.Context, orderID string, checkout UpdateCheckoutInput) (*domain.Order, error)
	createWebhookEventFn       func(ctx context.Context, event domain.PaymentWebhookEvent) (*domain.PaymentWebhookEvent, bool, error)
	applyWebhookResultFn       func(ctx context.Context, input ApplyWebhookResultInput) (*domain.Order, error)
	ensureOutboxEventFn        func(ctx context.Context, event domain.PaymentOutboxEvent) error
	claimOutboxEventsFn        func(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentOutboxEvent, error)
	markOutboxEventPublishedFn func(ctx context.Context, eventID string, publishedAt time.Time) error
	releaseOutboxEventFn       func(ctx context.Context, eventID string, availableAt time.Time, lastError string) error
	listOpenOrdersFn           func(ctx context.Context, provider domain.Provider, now, olderThan time.Time, limit int) ([]domain.Order, error)
	listExpiredOrdersFn        func(ctx context.Context, now time.Time, limit int) ([]domain.Order, error)
	expireOrderFn              func(ctx context.Context, orderID string, providerStatus string, failedAt time.Time) (*domain.Order, error)
	hasOtherOpenOrderFn        func(ctx context.Context, userID, courseID, excludeOrderID string) (bool, error)
	enqueueWebhookJobFn        func(ctx context.Context, job domain.PaymentWebhookJob) (*domain.PaymentWebhookJob, bool, error)
	claimWebhookJobByIDFn      func(ctx context.Context, jobID, workerID string, now time.Time) (*domain.PaymentWebhookJob, error)
	claimWebhookJobsFn         func(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentWebhookJob, error)
	markWebhookJobProcessedFn  func(ctx context.Context, jobID string, processedAt time.Time) error
	releaseWebhookJobFn        func(ctx context.Context, jobID string, availableAt time.Time, lastError string) error
	upsertOrderIssueFn         func(ctx context.Context, issue domain.PaymentOrderIssue) error
}

func (m *mockOrderRepository) Create(ctx context.Context, order domain.Order) (*domain.Order, error) {
	return m.createFn(ctx, order)
}

func (m *mockOrderRepository) GetByID(ctx context.Context, orderID string) (*domain.Order, error) {
	return m.getByIDFn(ctx, orderID)
}

func (m *mockOrderRepository) GetByIdempotencyKey(ctx context.Context, userID, courseID string, provider domain.Provider, idempotencyKey string) (*domain.Order, error) {
	return m.getByIdempotencyKeyFn(ctx, userID, courseID, provider, idempotencyKey)
}

func (m *mockOrderRepository) GetOpenByUserCourse(ctx context.Context, userID, courseID string) (*domain.Order, error) {
	if m.getOpenByUserCourseFn == nil {
		return nil, ErrOrderNotFound
	}
	return m.getOpenByUserCourseFn(ctx, userID, courseID)
}

func (m *mockOrderRepository) UpdateCheckout(ctx context.Context, orderID string, checkout UpdateCheckoutInput) (*domain.Order, error) {
	return m.updateCheckoutFn(ctx, orderID, checkout)
}

func (m *mockOrderRepository) CreateWebhookEvent(ctx context.Context, event domain.PaymentWebhookEvent) (*domain.PaymentWebhookEvent, bool, error) {
	return m.createWebhookEventFn(ctx, event)
}

func (m *mockOrderRepository) ApplyWebhookResult(ctx context.Context, input ApplyWebhookResultInput) (*domain.Order, error) {
	return m.applyWebhookResultFn(ctx, input)
}

func (m *mockOrderRepository) EnsureOutboxEvent(ctx context.Context, event domain.PaymentOutboxEvent) error {
	if m.ensureOutboxEventFn == nil {
		return nil
	}
	return m.ensureOutboxEventFn(ctx, event)
}

func (m *mockOrderRepository) ClaimOutboxEvents(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentOutboxEvent, error) {
	if m.claimOutboxEventsFn == nil {
		return nil, nil
	}
	return m.claimOutboxEventsFn(ctx, workerID, now, limit)
}

func (m *mockOrderRepository) MarkOutboxEventPublished(ctx context.Context, eventID string, publishedAt time.Time) error {
	if m.markOutboxEventPublishedFn == nil {
		return nil
	}
	return m.markOutboxEventPublishedFn(ctx, eventID, publishedAt)
}

func (m *mockOrderRepository) ReleaseOutboxEvent(ctx context.Context, eventID string, availableAt time.Time, lastError string) error {
	if m.releaseOutboxEventFn == nil {
		return nil
	}
	return m.releaseOutboxEventFn(ctx, eventID, availableAt, lastError)
}

func (m *mockOrderRepository) ListOpenOrdersForReconciliation(ctx context.Context, provider domain.Provider, now, olderThan time.Time, limit int) ([]domain.Order, error) {
	if m.listOpenOrdersFn == nil {
		return nil, nil
	}
	return m.listOpenOrdersFn(ctx, provider, now, olderThan, limit)
}

func (m *mockOrderRepository) ListExpiredOpenOrders(ctx context.Context, now time.Time, limit int) ([]domain.Order, error) {
	if m.listExpiredOrdersFn == nil {
		return nil, nil
	}
	return m.listExpiredOrdersFn(ctx, now, limit)
}

func (m *mockOrderRepository) ExpireOrder(ctx context.Context, orderID string, providerStatus string, failedAt time.Time) (*domain.Order, error) {
	return m.expireOrderFn(ctx, orderID, providerStatus, failedAt)
}

func (m *mockOrderRepository) HasOtherOpenOrder(ctx context.Context, userID, courseID, excludeOrderID string) (bool, error) {
	if m.hasOtherOpenOrderFn == nil {
		return false, nil
	}
	return m.hasOtherOpenOrderFn(ctx, userID, courseID, excludeOrderID)
}

func (m *mockOrderRepository) EnqueueWebhookJob(ctx context.Context, job domain.PaymentWebhookJob) (*domain.PaymentWebhookJob, bool, error) {
	if m.enqueueWebhookJobFn == nil {
		return &job, true, nil
	}
	return m.enqueueWebhookJobFn(ctx, job)
}

func (m *mockOrderRepository) ClaimWebhookJobByID(ctx context.Context, jobID, workerID string, now time.Time) (*domain.PaymentWebhookJob, error) {
	if m.claimWebhookJobByIDFn == nil {
		return nil, ErrOrderNotFound
	}
	return m.claimWebhookJobByIDFn(ctx, jobID, workerID, now)
}

func (m *mockOrderRepository) ClaimWebhookJobs(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentWebhookJob, error) {
	if m.claimWebhookJobsFn == nil {
		return nil, nil
	}
	return m.claimWebhookJobsFn(ctx, workerID, now, limit)
}

func (m *mockOrderRepository) MarkWebhookJobProcessed(ctx context.Context, jobID string, processedAt time.Time) error {
	if m.markWebhookJobProcessedFn == nil {
		return nil
	}
	return m.markWebhookJobProcessedFn(ctx, jobID, processedAt)
}

func (m *mockOrderRepository) ReleaseWebhookJob(ctx context.Context, jobID string, availableAt time.Time, lastError string) error {
	if m.releaseWebhookJobFn == nil {
		return nil
	}
	return m.releaseWebhookJobFn(ctx, jobID, availableAt, lastError)
}

func (m *mockOrderRepository) UpsertOrderIssue(ctx context.Context, issue domain.PaymentOrderIssue) error {
	if m.upsertOrderIssueFn == nil {
		return nil
	}
	return m.upsertOrderIssueFn(ctx, issue)
}

type fakeEnrollmentsClient struct {
	hasPendingFn    func(ctx context.Context, userID, courseID string) (bool, error)
	cancelPendingFn func(ctx context.Context, userID, courseID string) error
}

func (f fakeEnrollmentsClient) HasPendingEnrollment(ctx context.Context, userID, courseID string) (bool, error) {
	return f.hasPendingFn(ctx, userID, courseID)
}

func (f fakeEnrollmentsClient) CancelPendingEnrollment(ctx context.Context, userID, courseID string) error {
	if f.cancelPendingFn == nil {
		return nil
	}
	return f.cancelPendingFn(ctx, userID, courseID)
}

type fakeCoursesClient struct {
	getCourseFn func(ctx context.Context, courseID string) (*CourseInfo, error)
}

func (f fakeCoursesClient) GetCourse(ctx context.Context, courseID string) (*CourseInfo, error) {
	return f.getCourseFn(ctx, courseID)
}

type fakePublisher struct {
	publishFn func(ctx context.Context, event PaymentPaidEvent) error
}

func (f fakePublisher) PublishPaymentPaid(ctx context.Context, event PaymentPaidEvent) error {
	if f.publishFn == nil {
		return nil
	}
	return f.publishFn(ctx, event)
}

type fakeCheckoutProvider struct {
	createCheckoutFn   func(ctx context.Context, input CreateCheckoutSessionInput) (*CheckoutSession, error)
	reconcilePaymentFn func(ctx context.Context, input ReconcilePaymentInput) (*ProcessWebhookInput, error)
	reconcileOrderFn   func(ctx context.Context, order *domain.Order) (*ProcessWebhookInput, error)
	isAllowedURLFn     func(checkoutURL string) bool
}

func (f fakeCheckoutProvider) CreateCheckout(ctx context.Context, input CreateCheckoutSessionInput) (*CheckoutSession, error) {
	return f.createCheckoutFn(ctx, input)
}

func (f fakeCheckoutProvider) ReconcilePayment(ctx context.Context, input ReconcilePaymentInput) (*ProcessWebhookInput, error) {
	return f.reconcilePaymentFn(ctx, input)
}

func (f fakeCheckoutProvider) ReconcileOpenOrder(ctx context.Context, order *domain.Order) (*ProcessWebhookInput, error) {
	if f.reconcileOrderFn == nil {
		return nil, nil
	}
	return f.reconcileOrderFn(ctx, order)
}

func (f fakeCheckoutProvider) IsCheckoutURLAllowed(checkoutURL string) bool {
	if f.isAllowedURLFn == nil {
		return true
	}
	return f.isAllowedURLFn(checkoutURL)
}

func TestCreateOrderWhenPendingCreatesMercadoPagoCheckout(t *testing.T) {
	t.Parallel()

	var created domain.Order
	var updatedCheckout UpdateCheckoutInput
	svc := NewPaymentsService(
		&mockOrderRepository{
			createFn: func(_ context.Context, order domain.Order) (*domain.Order, error) {
				created = order
				return &order, nil
			},
			getByIDFn: func(context.Context, string) (*domain.Order, error) {
				return nil, ErrOrderNotFound
			},
			getByIdempotencyKeyFn: func(context.Context, string, string, domain.Provider, string) (*domain.Order, error) {
				return nil, ErrOrderNotFound
			},
			getOpenByUserCourseFn: func(context.Context, string, string) (*domain.Order, error) {
				return nil, ErrOrderNotFound
			},
			updateCheckoutFn: func(_ context.Context, orderID string, checkout UpdateCheckoutInput) (*domain.Order, error) {
				require.Equal(t, created.ID, orderID)
				updatedCheckout = checkout
				return &domain.Order{
					ID:                   orderID,
					UserID:               created.UserID,
					CourseID:             created.CourseID,
					AmountCents:          created.AmountCents,
					Currency:             created.Currency,
					Provider:             created.Provider,
					Status:               created.Status,
					IdempotencyKey:       created.IdempotencyKey,
					CheckoutURL:          &checkout.CheckoutURL,
					ExternalReference:    &checkout.ExternalReference,
					ProviderPreferenceID: &checkout.ProviderReferenceID,
					ProviderStatus:       checkout.ProviderStatus,
					ExpiresAt:            created.ExpiresAt,
				}, nil
			},
			createWebhookEventFn: func(context.Context, domain.PaymentWebhookEvent) (*domain.PaymentWebhookEvent, bool, error) {
				return nil, false, nil
			},
			applyWebhookResultFn: func(context.Context, ApplyWebhookResultInput) (*domain.Order, error) {
				return nil, nil
			},
			expireOrderFn: func(context.Context, string, string, time.Time) (*domain.Order, error) {
				return nil, nil
			},
		},
		fakeEnrollmentsClient{
			hasPendingFn: func(context.Context, string, string) (bool, error) {
				return true, nil
			},
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) {
				return &CourseInfo{
					ID:         "course-1",
					Title:      "Finanzas Personales",
					PriceCents: 1999900,
					Currency:   "ARS",
				}, nil
			},
		},
		fakePublisher{},
		map[domain.Provider]CheckoutProvider{
			domain.ProviderMercadoPago: fakeCheckoutProvider{
				createCheckoutFn: func(_ context.Context, input CreateCheckoutSessionInput) (*CheckoutSession, error) {
					require.Equal(t, "Finanzas Personales", input.CourseTitle)
					require.Equal(t, int64(1999900), input.AmountCents)
					require.NotNil(t, input.ExpiresAt)
					return &CheckoutSession{
						CheckoutURL:         "https://www.mercadopago.com/checkout/v1/redirect?pref_id=test",
						ExternalReference:   input.OrderID,
						ProviderReferenceID: "pref-123",
					}, nil
				},
			},
		},
		0,
		0,
	)
	frozenNow := time.Date(2026, time.March, 12, 20, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return frozenNow }

	result, err := svc.CreateOrder(context.Background(), "student-1", CreateOrderInput{
		CourseID: "course-1",
		Provider: domain.ProviderMercadoPago,
	}, "idem-1")
	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, domain.StatusCreated, created.Status)
	require.Equal(t, int64(1999900), created.AmountCents)
	require.Equal(t, "ARS", created.Currency)
	require.Equal(t, "https://www.mercadopago.com/checkout/v1/redirect?pref_id=test", updatedCheckout.CheckoutURL)
	require.Equal(t, created.ID, updatedCheckout.ExternalReference)
	require.Equal(t, "pref-123", updatedCheckout.ProviderReferenceID)
	require.NotNil(t, created.ExpiresAt)
}

func TestGetOrderRefreshesStaleCheckoutURL(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 13, 18, 0, 0, 0, time.UTC)
	order := &domain.Order{
		ID:             "order-1",
		UserID:         "user-1",
		CourseID:       "course-1",
		AmountCents:    150000,
		Currency:       "ARS",
		Provider:       domain.ProviderMercadoPago,
		Status:         domain.StatusCreated,
		IdempotencyKey: "idem-1",
		CheckoutURL:    ptr("https://sandbox.mercadopago.com.ar/checkout/v1/redirect?pref_id=old"),
		ExpiresAt:      ptrTime(now.Add(30 * time.Minute)),
	}

	var updated UpdateCheckoutInput

	svc := NewPaymentsService(
		&mockOrderRepository{
			getByIDFn: func(context.Context, string) (*domain.Order, error) {
				cloned := *order
				return &cloned, nil
			},
			updateCheckoutFn: func(_ context.Context, orderID string, checkout UpdateCheckoutInput) (*domain.Order, error) {
				require.Equal(t, order.ID, orderID)
				updated = checkout
				cloned := *order
				cloned.CheckoutURL = &checkout.CheckoutURL
				cloned.ExternalReference = &checkout.ExternalReference
				cloned.ProviderPreferenceID = &checkout.ProviderReferenceID
				cloned.ProviderStatus = checkout.ProviderStatus
				return &cloned, nil
			},
			expireOrderFn: func(context.Context, string, string, time.Time) (*domain.Order, error) {
				t.Fatal("order should not expire")
				return nil, nil
			},
		},
		fakeEnrollmentsClient{},
		fakeCoursesClient{
			getCourseFn: func(_ context.Context, courseID string) (*CourseInfo, error) {
				require.Equal(t, order.CourseID, courseID)
				return &CourseInfo{
					ID:         courseID,
					Title:      "Backend con Go",
					PriceCents: 150000,
					Currency:   "ARS",
				}, nil
			},
		},
		fakePublisher{},
		map[domain.Provider]CheckoutProvider{
			domain.ProviderMercadoPago: fakeCheckoutProvider{
				isAllowedURLFn: func(checkoutURL string) bool {
					return checkoutURL == "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=new"
				},
				createCheckoutFn: func(context.Context, CreateCheckoutSessionInput) (*CheckoutSession, error) {
					return &CheckoutSession{
						CheckoutURL:         "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=new",
						ExternalReference:   order.ID,
						ProviderReferenceID: "pref-new",
					}, nil
				},
			},
		},
		30*time.Minute,
		72*time.Hour,
	)
	svc.now = func() time.Time { return now }

	got, err := svc.GetOrder(context.Background(), order.UserID, order.ID)
	require.NoError(t, err)
	require.NotNil(t, got.CheckoutURL)
	require.Equal(t, "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=new", *got.CheckoutURL)
	require.Equal(t, "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=new", updated.CheckoutURL)
}

func TestCreateOrderFailsWithoutPendingEnrollment(t *testing.T) {
	t.Parallel()

	svc := NewPaymentsService(
		&mockOrderRepository{
			createFn:  func(context.Context, domain.Order) (*domain.Order, error) { return nil, nil },
			getByIDFn: func(context.Context, string) (*domain.Order, error) { return nil, ErrOrderNotFound },
			getByIdempotencyKeyFn: func(context.Context, string, string, domain.Provider, string) (*domain.Order, error) {
				return nil, ErrOrderNotFound
			},
			getOpenByUserCourseFn: func(context.Context, string, string) (*domain.Order, error) {
				return nil, ErrOrderNotFound
			},
			updateCheckoutFn: func(context.Context, string, UpdateCheckoutInput) (*domain.Order, error) {
				return nil, nil
			},
			createWebhookEventFn: func(context.Context, domain.PaymentWebhookEvent) (*domain.PaymentWebhookEvent, bool, error) {
				return nil, false, nil
			},
			applyWebhookResultFn: func(context.Context, ApplyWebhookResultInput) (*domain.Order, error) {
				return nil, nil
			},
			expireOrderFn: func(context.Context, string, string, time.Time) (*domain.Order, error) {
				return nil, nil
			},
		},
		fakeEnrollmentsClient{
			hasPendingFn: func(context.Context, string, string) (bool, error) {
				return false, nil
			},
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) {
				return &CourseInfo{ID: "course-1", PriceCents: 9950, Currency: "USD"}, nil
			},
		},
		fakePublisher{},
		map[domain.Provider]CheckoutProvider{
			domain.ProviderMercadoPago: fakeCheckoutProvider{
				createCheckoutFn: func(context.Context, CreateCheckoutSessionInput) (*CheckoutSession, error) {
					return nil, nil
				},
			},
		},
		0,
		0,
	)

	_, err := svc.CreateOrder(context.Background(), "student-1", CreateOrderInput{
		CourseID: "course-1",
		Provider: domain.ProviderMercadoPago,
	}, "idem-1")
	require.ErrorIs(t, err, ErrPendingEnrollmentRequired)
}

func TestWebhookIdempotencyEnqueuesOutboxOnce(t *testing.T) {
	t.Parallel()

	order := &domain.Order{
		ID:          "order-1",
		UserID:      "student-1",
		CourseID:    "course-1",
		AmountCents: 1000,
		Currency:    "USD",
		Provider:    domain.ProviderMercadoPago,
		Status:      domain.StatusCreated,
		CheckoutURL: ptr("https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=current"),
	}
	outboxCount := 0
	webhookCreated := false

	svc := NewPaymentsService(
		&mockOrderRepository{
			createFn: func(context.Context, domain.Order) (*domain.Order, error) { return nil, nil },
			getByIDFn: func(_ context.Context, orderID string) (*domain.Order, error) {
				require.Equal(t, "order-1", orderID)
				return order, nil
			},
			getByIdempotencyKeyFn: func(context.Context, string, string, domain.Provider, string) (*domain.Order, error) {
				return nil, ErrOrderNotFound
			},
			updateCheckoutFn: func(context.Context, string, UpdateCheckoutInput) (*domain.Order, error) {
				return nil, nil
			},
			createWebhookEventFn: func(_ context.Context, event domain.PaymentWebhookEvent) (*domain.PaymentWebhookEvent, bool, error) {
				if webhookCreated {
					now := time.Now().UTC()
					return &domain.PaymentWebhookEvent{
						ID:          event.ID,
						Provider:    event.Provider,
						EventKey:    event.EventKey,
						ProcessedAt: &now,
					}, false, nil
				}
				webhookCreated = true
				return &event, true, nil
			},
			applyWebhookResultFn: func(_ context.Context, input ApplyWebhookResultInput) (*domain.Order, error) {
				if input.OutboxEvent != nil {
					outboxCount++
				}
				order = &domain.Order{
					ID:                input.Update.OrderID,
					UserID:            "student-1",
					CourseID:          "course-1",
					AmountCents:       1000,
					Currency:          "USD",
					Provider:          domain.ProviderMercadoPago,
					ProviderPaymentID: input.Update.ProviderPaymentID,
					ProviderStatus:    input.Update.ProviderStatus,
					Status:            input.Update.Status,
					LastWebhookAt:     input.Update.LastWebhookAt,
					PaidAt:            input.Update.PaidAt,
					UpdatedAt:         input.Update.UpdatedAt,
				}
				return order, nil
			},
			ensureOutboxEventFn: func(context.Context, domain.PaymentOutboxEvent) error {
				outboxCount++
				return nil
			},
			expireOrderFn: func(context.Context, string, string, time.Time) (*domain.Order, error) {
				return nil, nil
			},
		},
		fakeEnrollmentsClient{
			hasPendingFn: func(context.Context, string, string) (bool, error) { return true, nil },
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) { return &CourseInfo{}, nil },
		},
		fakePublisher{},
		map[domain.Provider]CheckoutProvider{},
		0,
		0,
	)

	paymentID := "pay-1"
	providerStatus := "approved"
	amountCents := int64(1000)
	currency := "USD"

	first, err := svc.ProcessWebhook(context.Background(), ProcessWebhookInput{
		EventKey:          "pay-1:approved",
		OrderID:           "order-1",
		Provider:          domain.ProviderMercadoPago,
		ProviderPaymentID: &paymentID,
		Status:            providerStatus,
		ProviderStatus:    &providerStatus,
		AmountCents:       &amountCents,
		Currency:          &currency,
		ReceivedAt:        time.Date(2026, time.March, 12, 20, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.True(t, first.Published)
	require.Equal(t, 1, outboxCount)

	second, err := svc.ProcessWebhook(context.Background(), ProcessWebhookInput{
		EventKey:          "pay-1:approved",
		OrderID:           "order-1",
		Provider:          domain.ProviderMercadoPago,
		ProviderPaymentID: &paymentID,
		Status:            providerStatus,
		ProviderStatus:    &providerStatus,
		AmountCents:       &amountCents,
		Currency:          &currency,
		ReceivedAt:        time.Date(2026, time.March, 12, 20, 0, 2, 0, time.UTC),
	})
	require.NoError(t, err)
	require.False(t, second.Published)
	require.True(t, second.Duplicate)
	require.Equal(t, 2, outboxCount)
}

func TestRefreshOrderReconcilesApprovedPayment(t *testing.T) {
	t.Parallel()

	order := &domain.Order{
		ID:          "order-1",
		UserID:      "student-1",
		CourseID:    "course-1",
		AmountCents: 1000,
		Currency:    "USD",
		Provider:    domain.ProviderMercadoPago,
		Status:      domain.StatusCreated,
		CheckoutURL: ptr("https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=current"),
	}
	outboxCount := 0

	svc := NewPaymentsService(
		&mockOrderRepository{
			createFn: func(context.Context, domain.Order) (*domain.Order, error) { return nil, nil },
			getByIDFn: func(_ context.Context, orderID string) (*domain.Order, error) {
				require.Equal(t, "order-1", orderID)
				return order, nil
			},
			getByIdempotencyKeyFn: func(context.Context, string, string, domain.Provider, string) (*domain.Order, error) {
				return nil, ErrOrderNotFound
			},
			updateCheckoutFn: func(context.Context, string, UpdateCheckoutInput) (*domain.Order, error) {
				return nil, nil
			},
			createWebhookEventFn: func(_ context.Context, event domain.PaymentWebhookEvent) (*domain.PaymentWebhookEvent, bool, error) {
				return &event, true, nil
			},
			applyWebhookResultFn: func(_ context.Context, input ApplyWebhookResultInput) (*domain.Order, error) {
				if input.OutboxEvent != nil {
					outboxCount++
				}
				order = &domain.Order{
					ID:                input.Update.OrderID,
					UserID:            "student-1",
					CourseID:          "course-1",
					AmountCents:       1000,
					Currency:          "USD",
					Provider:          domain.ProviderMercadoPago,
					ProviderPaymentID: input.Update.ProviderPaymentID,
					ProviderStatus:    input.Update.ProviderStatus,
					Status:            input.Update.Status,
					LastWebhookAt:     input.Update.LastWebhookAt,
					PaidAt:            input.Update.PaidAt,
					UpdatedAt:         input.Update.UpdatedAt,
				}
				return order, nil
			},
			expireOrderFn: func(context.Context, string, string, time.Time) (*domain.Order, error) {
				return nil, nil
			},
		},
		fakeEnrollmentsClient{
			hasPendingFn: func(context.Context, string, string) (bool, error) { return true, nil },
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) { return &CourseInfo{}, nil },
		},
		fakePublisher{},
		map[domain.Provider]CheckoutProvider{
			domain.ProviderMercadoPago: fakeCheckoutProvider{
				isAllowedURLFn: func(checkoutURL string) bool {
					return checkoutURL == "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=current"
				},
				createCheckoutFn: func(context.Context, CreateCheckoutSessionInput) (*CheckoutSession, error) {
					return nil, nil
				},
				reconcilePaymentFn: func(_ context.Context, input ReconcilePaymentInput) (*ProcessWebhookInput, error) {
					require.Equal(t, "pay-123", input.PaymentID)
					require.Equal(t, "order-1", input.Order.ID)
					paymentID := "pay-123"
					providerStatus := "approved"
					amountCents := int64(1000)
					currency := "USD"
					return &ProcessWebhookInput{
						EventKey:          "pay-123:approved",
						OrderID:           "order-1",
						Provider:          domain.ProviderMercadoPago,
						ProviderPaymentID: &paymentID,
						Status:            providerStatus,
						ProviderStatus:    &providerStatus,
						ResourceID:        &paymentID,
						AmountCents:       &amountCents,
						Currency:          &currency,
						ReceivedAt:        time.Date(2026, time.March, 12, 20, 0, 0, 0, time.UTC),
					}, nil
				},
			},
		},
		0,
		0,
	)

	refreshed, err := svc.RefreshOrder(context.Background(), "student-1", "order-1", RefreshOrderInput{
		PaymentID: "pay-123",
	})
	require.NoError(t, err)
	require.Equal(t, domain.StatusPaid, refreshed.Status)
	require.Equal(t, "approved", *refreshed.ProviderStatus)
	require.Equal(t, "pay-123", *refreshed.ProviderPaymentID)
	require.Equal(t, 1, outboxCount)
}

func TestProcessWebhookKeepsOriginalProviderPaymentWhenDifferentPaidPaymentArrives(t *testing.T) {
	t.Parallel()

	order := &domain.Order{
		ID:                "order-1",
		UserID:            "student-1",
		CourseID:          "course-1",
		AmountCents:       1000,
		Currency:          "USD",
		Provider:          domain.ProviderMercadoPago,
		Status:            domain.StatusPaid,
		ProviderPaymentID: ptr("pay-original"),
		ProviderStatus:    ptr("approved"),
		PaidAt:            ptrTime(time.Date(2026, time.March, 12, 20, 0, 0, 0, time.UTC)),
	}
	issueRecorded := false

	svc := NewPaymentsService(
		&mockOrderRepository{
			getByIDFn: func(context.Context, string) (*domain.Order, error) {
				return order, nil
			},
			createWebhookEventFn: func(_ context.Context, event domain.PaymentWebhookEvent) (*domain.PaymentWebhookEvent, bool, error) {
				return &event, true, nil
			},
			applyWebhookResultFn: func(_ context.Context, input ApplyWebhookResultInput) (*domain.Order, error) {
				require.Equal(t, domain.StatusPaid, input.Update.Status)
				require.NotNil(t, input.Update.ProviderPaymentID)
				require.Equal(t, "pay-original", *input.Update.ProviderPaymentID)
				return order, nil
			},
			upsertOrderIssueFn: func(_ context.Context, issue domain.PaymentOrderIssue) error {
				issueRecorded = true
				require.Equal(t, "provider_payment_conflict", issue.IssueType)
				return nil
			},
			expireOrderFn: func(context.Context, string, string, time.Time) (*domain.Order, error) {
				return nil, nil
			},
		},
		fakeEnrollmentsClient{},
		fakeCoursesClient{},
		fakePublisher{},
		map[domain.Provider]CheckoutProvider{},
		0,
		0,
	)

	paymentID := "pay-duplicate"
	providerStatus := "approved"
	amountCents := int64(1000)
	currency := "USD"
	result, err := svc.ProcessWebhook(context.Background(), ProcessWebhookInput{
		EventKey:          "pay-duplicate:approved",
		OrderID:           "order-1",
		Provider:          domain.ProviderMercadoPago,
		ProviderPaymentID: &paymentID,
		Status:            providerStatus,
		ProviderStatus:    &providerStatus,
		AmountCents:       &amountCents,
		Currency:          &currency,
		ReceivedAt:        time.Date(2026, time.March, 12, 20, 5, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.True(t, issueRecorded)
	require.Equal(t, order.ID, result.Order.ID)
	require.False(t, result.Published)
}

func ptr(value string) *string {
	return &value
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
