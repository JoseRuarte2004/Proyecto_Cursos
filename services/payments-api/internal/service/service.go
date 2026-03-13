package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/services/payments-api/internal/domain"
)

var (
	ErrCourseIDRequired          = errors.New("courseId is required")
	ErrInvalidProvider           = errors.New("invalid provider")
	ErrPendingEnrollmentRequired = errors.New("pending enrollment is required")
	ErrOpenOrderAlreadyExists    = errors.New("open order already exists")
	ErrOrderNotFound             = errors.New("order not found")
	ErrWebhookStatusInvalid      = errors.New("invalid webhook status")
	ErrProviderMismatch          = errors.New("provider does not match order")
	ErrDuplicateIdempotencyKey   = errors.New("idempotency key already exists")
	ErrProviderUnsupported       = errors.New("provider is not configured")
	ErrProviderMisconfigured     = errors.New("provider is misconfigured")
	ErrProviderRequestFailed     = errors.New("provider request failed")
	ErrWebhookSignatureInvalid   = errors.New("invalid webhook signature")
	ErrWebhookPayloadInvalid     = errors.New("invalid webhook payload")
	ErrWebhookEventKeyRequired   = errors.New("webhook event key is required")
	ErrWebhookEventNotFound      = errors.New("webhook event not found")
	ErrPaymentAmountMismatch     = errors.New("payment amount does not match order")
	ErrPaymentCurrencyMismatch   = errors.New("payment currency does not match order")
	ErrPaymentConflict           = errors.New("payment conflicts with an existing provider payment")
)

type OrderRepository interface {
	Create(ctx context.Context, order domain.Order) (*domain.Order, error)
	GetByID(ctx context.Context, orderID string) (*domain.Order, error)
	GetByIdempotencyKey(ctx context.Context, userID, courseID string, provider domain.Provider, idempotencyKey string) (*domain.Order, error)
	GetOpenByUserCourse(ctx context.Context, userID, courseID string) (*domain.Order, error)
	UpdateCheckout(ctx context.Context, orderID string, checkout UpdateCheckoutInput) (*domain.Order, error)
	CreateWebhookEvent(ctx context.Context, event domain.PaymentWebhookEvent) (*domain.PaymentWebhookEvent, bool, error)
	ApplyWebhookResult(ctx context.Context, input ApplyWebhookResultInput) (*domain.Order, error)
	EnsureOutboxEvent(ctx context.Context, event domain.PaymentOutboxEvent) error
	ClaimOutboxEvents(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentOutboxEvent, error)
	MarkOutboxEventPublished(ctx context.Context, eventID string, publishedAt time.Time) error
	ReleaseOutboxEvent(ctx context.Context, eventID string, availableAt time.Time, lastError string) error
	ListOpenOrdersForReconciliation(ctx context.Context, provider domain.Provider, now, olderThan time.Time, limit int) ([]domain.Order, error)
	ListExpiredOpenOrders(ctx context.Context, now time.Time, limit int) ([]domain.Order, error)
	ExpireOrder(ctx context.Context, orderID string, providerStatus string, failedAt time.Time) (*domain.Order, error)
	HasOtherOpenOrder(ctx context.Context, userID, courseID, excludeOrderID string) (bool, error)
	EnqueueWebhookJob(ctx context.Context, job domain.PaymentWebhookJob) (*domain.PaymentWebhookJob, bool, error)
	ClaimWebhookJobByID(ctx context.Context, jobID, workerID string, now time.Time) (*domain.PaymentWebhookJob, error)
	ClaimWebhookJobs(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentWebhookJob, error)
	MarkWebhookJobProcessed(ctx context.Context, jobID string, processedAt time.Time) error
	ReleaseWebhookJob(ctx context.Context, jobID string, availableAt time.Time, lastError string) error
	UpsertOrderIssue(ctx context.Context, issue domain.PaymentOrderIssue) error
}

type EnrollmentsClient interface {
	HasPendingEnrollment(ctx context.Context, userID, courseID string) (bool, error)
	CancelPendingEnrollment(ctx context.Context, userID, courseID string) error
}

type CourseInfo struct {
	ID         string
	Title      string
	Category   string
	ImageURL   *string
	PriceCents int64
	Currency   string
	Status     string
}

type CoursesClient interface {
	GetCourse(ctx context.Context, courseID string) (*CourseInfo, error)
}

type PaymentPublisher interface {
	PublishPaymentPaid(ctx context.Context, event PaymentPaidEvent) error
}

type CheckoutProvider interface {
	CreateCheckout(ctx context.Context, input CreateCheckoutSessionInput) (*CheckoutSession, error)
}

type PaymentReconciler interface {
	ReconcilePayment(ctx context.Context, input ReconcilePaymentInput) (*ProcessWebhookInput, error)
}

type OpenOrderReconciler interface {
	ReconcileOpenOrder(ctx context.Context, order *domain.Order) (*ProcessWebhookInput, error)
}

type CheckoutURLValidator interface {
	IsCheckoutURLAllowed(checkoutURL string) bool
}

type CreateCheckoutSessionInput struct {
	OrderID     string
	CourseID    string
	CourseTitle string
	AmountCents int64
	Currency    string
	Provider    domain.Provider
	ExpiresAt   *time.Time
}

type CheckoutSession struct {
	CheckoutURL         string
	ExternalReference   string
	ProviderReferenceID string
	ProviderStatus      *string
}

type CreateOrderInput struct {
	CourseID string
	Provider domain.Provider
}

type CreateOrderResult struct {
	Order          *domain.Order
	CheckoutURL    string
	IdempotencyKey string
	Created        bool
}

type UpdateCheckoutInput struct {
	CheckoutURL         string
	ExternalReference   string
	ProviderReferenceID string
	ProviderStatus      *string
	UpdatedAt           time.Time
}

type UpdateOrderStatusInput struct {
	OrderID           string
	Status            domain.Status
	ProviderPaymentID *string
	ProviderStatus    *string
	LastWebhookAt     *time.Time
	PaidAt            *time.Time
	FailedAt          *time.Time
	ExpiresAt         *time.Time
	UpdatedAt         time.Time
}

type ProcessWebhookInput struct {
	EventKey          string
	OrderID           string
	Provider          domain.Provider
	ProviderPaymentID *string
	Status            string
	ProviderStatus    *string
	AmountCents       *int64
	Currency          *string
	RequestID         *string
	Topic             *string
	Action            *string
	ResourceID        *string
	ReceivedAt        time.Time
	Payload           string
}

type ProcessWebhookResult struct {
	Order     *domain.Order
	Published bool
	Duplicate bool
}

type QueueWebhookInput struct {
	Provider           domain.Provider
	DedupeKey          string
	ResourceID         string
	RequestID          *string
	SignatureTimestamp *string
	Topic              *string
	Action             *string
	ReceivedAt         time.Time
	Payload            string
}

type ReconcilePaymentInput struct {
	Order     *domain.Order
	PaymentID string
}

type RefreshOrderInput struct {
	PaymentID string
}

type ApplyWebhookResultInput struct {
	EventProvider domain.Provider
	EventKey      string
	EventOrderID  string
	ProcessedAt   time.Time
	Update        UpdateOrderStatusInput
	OutboxEvent   *domain.PaymentOutboxEvent
}

type PaymentPaidEvent struct {
	OrderID     string          `json:"orderId"`
	UserID      string          `json:"userId"`
	CourseID    string          `json:"courseId"`
	AmountCents int64           `json:"amountCents"`
	Currency    string          `json:"currency"`
	Provider    domain.Provider `json:"provider"`
}

type PaymentsService struct {
	repo            OrderRepository
	enrollments     EnrollmentsClient
	courses         CoursesClient
	publisher       PaymentPublisher
	providers       map[domain.Provider]CheckoutProvider
	orderCreatedTTL time.Duration
	orderPendingTTL time.Duration
	now             func() time.Time
}

func NewPaymentsService(repo OrderRepository, enrollments EnrollmentsClient, courses CoursesClient, publisher PaymentPublisher, providers map[domain.Provider]CheckoutProvider, orderCreatedTTL, orderPendingTTL time.Duration) *PaymentsService {
	if orderCreatedTTL <= 0 {
		orderCreatedTTL = 30 * time.Minute
	}
	if orderPendingTTL <= 0 {
		orderPendingTTL = 72 * time.Hour
	}
	return &PaymentsService{
		repo:            repo,
		enrollments:     enrollments,
		courses:         courses,
		publisher:       publisher,
		providers:       providers,
		orderCreatedTTL: orderCreatedTTL,
		orderPendingTTL: orderPendingTTL,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *PaymentsService) CreateOrder(ctx context.Context, userID string, input CreateOrderInput, idempotencyKey string) (*CreateOrderResult, error) {
	courseID := strings.TrimSpace(input.CourseID)
	if courseID == "" {
		return nil, ErrCourseIDRequired
	}
	if !input.Provider.IsValid() {
		return nil, ErrInvalidProvider
	}
	if provider := s.providers[input.Provider]; provider == nil {
		return nil, ErrProviderUnsupported
	}

	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		key = uuid.NewString()
	}

	scopedUserID := strings.TrimSpace(userID)
	existing, err := s.repo.GetByIdempotencyKey(ctx, scopedUserID, courseID, input.Provider, key)
	if err == nil {
		if existing.UserID != scopedUserID || existing.CourseID != courseID || existing.Provider != input.Provider {
			log.Printf(
				"[payments-api] idempotency scope mismatch rejected user_id=%s course_id=%s provider=%s idempotency_key=%s existing_order_id=%s existing_user_id=%s existing_course_id=%s existing_provider=%s",
				scopedUserID,
				courseID,
				input.Provider,
				key,
				existing.ID,
				existing.UserID,
				existing.CourseID,
				existing.Provider,
			)
			return nil, ErrDuplicateIdempotencyKey
		}
		readyOrder, readyCheckoutURL, ensureErr := s.ensureCheckout(ctx, existing, "")
		if ensureErr != nil {
			return nil, ensureErr
		}

		return &CreateOrderResult{
			Order:          readyOrder,
			CheckoutURL:    readyCheckoutURL,
			IdempotencyKey: key,
			Created:        false,
		}, nil
	}
	if !errors.Is(err, ErrOrderNotFound) {
		return nil, err
	}

	openOrder, err := s.repo.GetOpenByUserCourse(ctx, scopedUserID, courseID)
	if err == nil {
		now := s.now()
		if s.isOrderExpired(openOrder, now) {
			if _, expireErr := s.repo.ExpireOrder(ctx, openOrder.ID, "expired", now); expireErr != nil {
				return nil, expireErr
			}
		} else {
			readyOrder, readyCheckoutURL, ensureErr := s.ensureCheckout(ctx, openOrder, "")
			if ensureErr != nil {
				return nil, ensureErr
			}

			return &CreateOrderResult{
				Order:          readyOrder,
				CheckoutURL:    readyCheckoutURL,
				IdempotencyKey: key,
				Created:        false,
			}, nil
		}
	} else if !errors.Is(err, ErrOrderNotFound) {
		return nil, err
	}

	pending, err := s.enrollments.HasPendingEnrollment(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if !pending {
		return nil, ErrPendingEnrollmentRequired
	}

	course, err := s.courses.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	now := s.now()
	expiresAt := now.Add(s.orderCreatedTTL)
	order := domain.Order{
		ID:             uuid.NewString(),
		UserID:         scopedUserID,
		CourseID:       courseID,
		AmountCents:    course.PriceCents,
		Currency:       course.Currency,
		Provider:       input.Provider,
		Status:         domain.StatusCreated,
		IdempotencyKey: key,
		ExpiresAt:      &expiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	created, err := s.repo.Create(ctx, order)
	if err != nil {
		if errors.Is(err, ErrDuplicateIdempotencyKey) {
			existing, getErr := s.repo.GetByIdempotencyKey(ctx, scopedUserID, courseID, input.Provider, key)
			if getErr != nil {
				return nil, getErr
			}

			readyOrder, readyCheckoutURL, ensureErr := s.ensureCheckout(ctx, existing, "")
			if ensureErr != nil {
				return nil, ensureErr
			}

			return &CreateOrderResult{
				Order:          readyOrder,
				CheckoutURL:    readyCheckoutURL,
				IdempotencyKey: key,
				Created:        false,
			}, nil
		}
		if errors.Is(err, ErrOpenOrderAlreadyExists) {
			existing, getErr := s.repo.GetOpenByUserCourse(ctx, strings.TrimSpace(userID), courseID)
			if getErr != nil {
				return nil, getErr
			}

			readyOrder, readyCheckoutURL, ensureErr := s.ensureCheckout(ctx, existing, course.Title)
			if ensureErr != nil {
				return nil, ensureErr
			}

			return &CreateOrderResult{
				Order:          readyOrder,
				CheckoutURL:    readyCheckoutURL,
				IdempotencyKey: key,
				Created:        false,
			}, nil
		}
		return nil, err
	}

	readyOrder, readyCheckoutURL, err := s.ensureCheckout(ctx, created, course.Title)
	if err != nil {
		return nil, err
	}

	return &CreateOrderResult{
		Order:          readyOrder,
		CheckoutURL:    readyCheckoutURL,
		IdempotencyKey: key,
		Created:        true,
	}, nil
}

func (s *PaymentsService) GetOrder(ctx context.Context, userID, orderID string) (*domain.Order, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, ErrOrderNotFound
	}

	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != strings.TrimSpace(userID) {
		return nil, ErrOrderNotFound
	}

	switch order.Status {
	case domain.StatusCreated, domain.StatusPending:
		readyOrder, _, ensureErr := s.ensureCheckout(ctx, order, "")
		if ensureErr == nil {
			return readyOrder, nil
		}
		if errors.Is(ensureErr, ErrPendingEnrollmentRequired) {
			refreshed, err := s.repo.GetByID(ctx, orderID)
			if err == nil && refreshed.UserID == strings.TrimSpace(userID) {
				return refreshed, nil
			}
		}
		return nil, ensureErr
	}

	return order, nil
}

func (s *PaymentsService) RefreshOrder(ctx context.Context, userID, orderID string, input RefreshOrderInput) (*domain.Order, error) {
	order, err := s.GetOrder(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}

	paymentID := strings.TrimSpace(input.PaymentID)
	if paymentID == "" {
		return order, nil
	}

	switch order.Status {
	case domain.StatusPaid, domain.StatusRefunded, domain.StatusFailed:
		return order, nil
	}

	provider, ok := s.providers[order.Provider]
	if !ok || provider == nil {
		return order, nil
	}

	reconciler, ok := provider.(PaymentReconciler)
	if !ok {
		return order, nil
	}

	log.Printf(
		"[payments-api] refresh order attempting reconciliation order_id=%s payment_id=%s provider=%s current_status=%s",
		order.ID,
		paymentID,
		order.Provider,
		order.Status,
	)

	reconciled, err := reconciler.ReconcilePayment(ctx, ReconcilePaymentInput{
		Order:     order,
		PaymentID: paymentID,
	})
	if err != nil {
		log.Printf(
			"[payments-api] refresh order reconciliation failed order_id=%s payment_id=%s provider=%s err=%v",
			order.ID,
			paymentID,
			order.Provider,
			err,
		)
		return nil, err
	}
	if strings.TrimSpace(reconciled.OrderID) != order.ID {
		log.Printf(
			"[payments-api] refresh order reconciliation mismatch expected_order_id=%s reconciled_order_id=%s payment_id=%s",
			order.ID,
			strings.TrimSpace(reconciled.OrderID),
			paymentID,
		)
		return nil, ErrWebhookPayloadInvalid
	}

	result, err := s.ProcessWebhook(ctx, *reconciled)
	if err != nil {
		log.Printf(
			"[payments-api] refresh order process failed order_id=%s payment_id=%s provider=%s err=%v",
			order.ID,
			paymentID,
			order.Provider,
			err,
		)
		return nil, err
	}

	return result.Order, nil
}

func (s *PaymentsService) ProcessWebhook(ctx context.Context, input ProcessWebhookInput) (*ProcessWebhookResult, error) {
	orderID := strings.TrimSpace(input.OrderID)
	if orderID == "" {
		log.Printf("[payments-api] process webhook failed reason=missing_order_id event_key=%s provider=%s", input.EventKey, input.Provider)
		return nil, ErrOrderNotFound
	}
	if !input.Provider.IsValid() {
		log.Printf("[payments-api] process webhook failed reason=invalid_provider order_id=%s provider=%s", orderID, input.Provider)
		return nil, ErrInvalidProvider
	}

	eventKey := strings.TrimSpace(input.EventKey)
	if eventKey == "" {
		log.Printf("[payments-api] process webhook failed reason=missing_event_key order_id=%s provider=%s", orderID, input.Provider)
		return nil, ErrWebhookEventKeyRequired
	}

	log.Printf(
		"[payments-api] process webhook start order_id=%s provider=%s payment_id=%s provider_status=%s event_key=%s resource_id=%s",
		orderID,
		input.Provider,
		derefString(input.ProviderPaymentID),
		derefString(input.ProviderStatus),
		eventKey,
		derefString(input.ResourceID),
	)

	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		log.Printf("[payments-api] process webhook order lookup failed order_id=%s err=%v", orderID, err)
		return nil, err
	}
	log.Printf(
		"[payments-api] process webhook order lookup success order_id=%s db_status=%s provider=%s provider_status=%s external_reference=%s",
		order.ID,
		order.Status,
		order.Provider,
		derefString(order.ProviderStatus),
		derefString(order.ExternalReference),
	)
	if order.Provider != input.Provider {
		log.Printf("[payments-api] process webhook provider mismatch order_id=%s order_provider=%s webhook_provider=%s", order.ID, order.Provider, input.Provider)
		return nil, ErrProviderMismatch
	}
	if input.AmountCents != nil && *input.AmountCents != order.AmountCents {
		log.Printf(
			"[payments-api] process webhook amount mismatch order_id=%s expected_cents=%d actual_cents=%d",
			order.ID,
			order.AmountCents,
			*input.AmountCents,
		)
		return nil, ErrPaymentAmountMismatch
	}
	if input.Currency != nil && !strings.EqualFold(strings.TrimSpace(*input.Currency), strings.TrimSpace(order.Currency)) {
		log.Printf(
			"[payments-api] process webhook currency mismatch order_id=%s expected=%s actual=%s",
			order.ID,
			order.Currency,
			derefString(input.Currency),
		)
		return nil, ErrPaymentCurrencyMismatch
	}

	now := s.now()
	event, created, err := s.repo.CreateWebhookEvent(ctx, domain.PaymentWebhookEvent{
		ID:          uuid.NewString(),
		Provider:    input.Provider,
		EventKey:    eventKey,
		RequestID:   normalizeOptionalString(input.RequestID),
		Topic:       normalizeOptionalString(input.Topic),
		Action:      normalizeOptionalString(input.Action),
		ResourceID:  normalizeOptionalString(input.ResourceID),
		OrderID:     &order.ID,
		Payload:     strings.TrimSpace(input.Payload),
		CreatedAt:   now,
		ProcessedAt: nil,
	})
	if err != nil {
		log.Printf("[payments-api] process webhook create event failed order_id=%s event_key=%s err=%v", order.ID, eventKey, err)
		return nil, err
	}
	if !created && event.ProcessedAt != nil {
		log.Printf("[payments-api] process webhook duplicate processed event order_id=%s event_key=%s", order.ID, eventKey)
		if order.Status == domain.StatusPaid {
			if err := s.repo.EnsureOutboxEvent(ctx, buildPaymentPaidOutboxEvent(order, now)); err != nil {
				return nil, err
			}
		}
		return &ProcessWebhookResult{Order: order, Published: false, Duplicate: true}, nil
	}
	if !created {
		log.Printf("[payments-api] process webhook retrying unprocessed event order_id=%s event_key=%s", order.ID, eventKey)
	}

	receivedAt := input.ReceivedAt.UTC()
	if receivedAt.IsZero() {
		receivedAt = now
	}

	if hasConflictingProviderPayment(order, input.ProviderPaymentID) {
		if err := s.repo.UpsertOrderIssue(ctx, buildProviderPaymentConflictIssue(order, input, now)); err != nil {
			return nil, err
		}
		log.Printf(
			"[payments-api] process webhook detected provider payment conflict order_id=%s existing_payment_id=%s incoming_payment_id=%s current_status=%s event_key=%s",
			order.ID,
			derefString(order.ProviderPaymentID),
			derefString(input.ProviderPaymentID),
			order.Status,
			eventKey,
		)
		if order.Status == domain.StatusPaid || order.Status == domain.StatusRefunded || order.Status == domain.StatusFailed {
			updated, err := s.repo.ApplyWebhookResult(ctx, ApplyWebhookResultInput{
				EventProvider: input.Provider,
				EventKey:      eventKey,
				EventOrderID:  order.ID,
				ProcessedAt:   now,
				Update: UpdateOrderStatusInput{
					OrderID:           order.ID,
					Status:            order.Status,
					ProviderPaymentID: normalizeOptionalString(order.ProviderPaymentID),
					ProviderStatus:    normalizeOptionalString(order.ProviderStatus),
					LastWebhookAt:     &receivedAt,
					PaidAt:            order.PaidAt,
					FailedAt:          order.FailedAt,
					ExpiresAt:         order.ExpiresAt,
					UpdatedAt:         now,
				},
			})
			if err != nil {
				return nil, err
			}
			return &ProcessWebhookResult{
				Order:     updated,
				Published: false,
				Duplicate: false,
			}, nil
		}
		return nil, ErrPaymentConflict
	}

	nextStatus, err := mapWebhookStatus(input.Status)
	if err != nil {
		log.Printf("[payments-api] process webhook status mapping failed order_id=%s provider_status=%s err=%v", order.ID, input.Status, err)
		return nil, err
	}
	finalStatus := reconcileStatus(order.Status, nextStatus)

	updateInput := UpdateOrderStatusInput{
		OrderID:           order.ID,
		Status:            finalStatus,
		ProviderPaymentID: normalizeOptionalString(input.ProviderPaymentID),
		ProviderStatus:    resolveProviderStatus(order, nextStatus, finalStatus, input.ProviderStatus, &input.Status),
		LastWebhookAt:     &receivedAt,
		UpdatedAt:         now,
	}

	shouldPublish := order.Status != domain.StatusPaid && finalStatus == domain.StatusPaid
	if shouldPublish {
		updateInput.PaidAt = &receivedAt
		updateInput.ExpiresAt = nil
	}
	if finalStatus == domain.StatusFailed && order.Status != domain.StatusFailed {
		updateInput.FailedAt = &receivedAt
		updateInput.ExpiresAt = nil
	}
	if finalStatus == domain.StatusPending {
		expiresAt := receivedAt.Add(s.orderPendingTTL)
		updateInput.ExpiresAt = &expiresAt
	}

	log.Printf(
		"[payments-api] process webhook updating order order_id=%s current_status=%s next_status=%s final_status=%s payment_id=%s provider_status=%s",
		order.ID,
		order.Status,
		nextStatus,
		finalStatus,
		derefString(input.ProviderPaymentID),
		derefString(updateInput.ProviderStatus),
	)

	var outboxEvent *domain.PaymentOutboxEvent
	if shouldPublish {
		event := buildPaymentPaidOutboxEvent(order, now)
		outboxEvent = &event
	}

	updated, err := s.repo.ApplyWebhookResult(ctx, ApplyWebhookResultInput{
		EventProvider: input.Provider,
		EventKey:      eventKey,
		EventOrderID:  order.ID,
		ProcessedAt:   now,
		Update:        updateInput,
		OutboxEvent:   outboxEvent,
	})
	if err != nil {
		log.Printf("[payments-api] process webhook update failed order_id=%s err=%v", order.ID, err)
		return nil, err
	}
	log.Printf(
		"[payments-api] process webhook update success order_id=%s new_status=%s provider_status=%s payment_id=%s",
		updated.ID,
		updated.Status,
		derefString(updated.ProviderStatus),
		derefString(updated.ProviderPaymentID),
	)

	result := &ProcessWebhookResult{
		Order: updated,
	}
	result.Published = shouldPublish

	return result, nil
}

func (s *PaymentsService) ensureCheckout(ctx context.Context, order *domain.Order, courseTitle string) (*domain.Order, string, error) {
	now := s.now()
	if s.isOrderExpired(order, now) {
		expired, err := s.repo.ExpireOrder(ctx, order.ID, "expired", now)
		if err != nil {
			return nil, "", err
		}
		return expired, "", ErrPendingEnrollmentRequired
	}

	provider, ok := s.providers[order.Provider]
	if !ok || provider == nil {
		return nil, "", ErrProviderUnsupported
	}

	if order.CheckoutURL != nil && strings.TrimSpace(*order.CheckoutURL) != "" {
		existingCheckoutURL := strings.TrimSpace(*order.CheckoutURL)
		if validator, ok := provider.(CheckoutURLValidator); !ok || validator.IsCheckoutURLAllowed(existingCheckoutURL) {
			return order, existingCheckoutURL, nil
		}
	}

	if strings.TrimSpace(courseTitle) == "" {
		course, err := s.courses.GetCourse(ctx, order.CourseID)
		if err != nil {
			return nil, "", err
		}
		courseTitle = course.Title
	}

	checkout, err := provider.CreateCheckout(ctx, CreateCheckoutSessionInput{
		OrderID:     order.ID,
		CourseID:    order.CourseID,
		CourseTitle: strings.TrimSpace(courseTitle),
		AmountCents: order.AmountCents,
		Currency:    order.Currency,
		Provider:    order.Provider,
		ExpiresAt:   order.ExpiresAt,
	})
	if err != nil {
		return nil, "", err
	}

	updated, err := s.repo.UpdateCheckout(ctx, order.ID, UpdateCheckoutInput{
		CheckoutURL:         strings.TrimSpace(checkout.CheckoutURL),
		ExternalReference:   strings.TrimSpace(checkout.ExternalReference),
		ProviderReferenceID: strings.TrimSpace(checkout.ProviderReferenceID),
		ProviderStatus:      normalizeOptionalString(checkout.ProviderStatus),
		UpdatedAt:           s.now(),
	})
	if err != nil {
		return nil, "", err
	}

	return updated, strings.TrimSpace(checkout.CheckoutURL), nil
}

func mapWebhookStatus(status string) (domain.Status, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "approved", "authorized", "paid", "succeeded", "success":
		return domain.StatusPaid, nil
	case "pending", "in_process", "in-process", "processing":
		return domain.StatusPending, nil
	case "failed", "rejected", "declined", "cancelled", "canceled":
		return domain.StatusFailed, nil
	case "refunded", "charged_back", "chargeback":
		return domain.StatusRefunded, nil
	default:
		return "", ErrWebhookStatusInvalid
	}
}

func reconcileStatus(current, incoming domain.Status) domain.Status {
	switch current {
	case domain.StatusRefunded:
		return domain.StatusRefunded
	case domain.StatusPaid:
		if incoming == domain.StatusRefunded {
			return domain.StatusRefunded
		}
		return domain.StatusPaid
	default:
		return incoming
	}
}

func resolveProviderStatus(order *domain.Order, nextStatus, finalStatus domain.Status, values ...*string) *string {
	if order != nil && finalStatus == order.Status && nextStatus != finalStatus {
		return normalizeOptionalString(order.ProviderStatus)
	}

	return firstNonEmptyPointer(values...)
}

func buildPaymentPaidOutboxEvent(order *domain.Order, now time.Time) domain.PaymentOutboxEvent {
	event := PaymentPaidEvent{
		OrderID:     order.ID,
		UserID:      order.UserID,
		CourseID:    order.CourseID,
		AmountCents: order.AmountCents,
		Currency:    order.Currency,
		Provider:    order.Provider,
	}
	payload := `{}`
	if encoded, err := json.Marshal(event); err == nil {
		payload = string(encoded)
	}

	return domain.PaymentOutboxEvent{
		ID:          uuid.NewString(),
		EventType:   domain.OutboxEventPaymentPaid,
		OrderID:     order.ID,
		Payload:     payload,
		Attempts:    0,
		CreatedAt:   now,
		AvailableAt: now,
	}
}

func (s *PaymentsService) isOrderExpired(order *domain.Order, now time.Time) bool {
	if order == nil || order.ExpiresAt == nil {
		return false
	}

	switch order.Status {
	case domain.StatusCreated, domain.StatusPending:
		return !order.ExpiresAt.After(now)
	default:
		return false
	}
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func firstNonEmptyPointer(values ...*string) *string {
	for _, value := range values {
		if normalized := normalizeOptionalString(value); normalized != nil {
			return normalized
		}
	}

	return nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return strings.TrimSpace(*value)
}

func (s *PaymentsService) DrainOutbox(ctx context.Context, workerID string, limit int) (int, error) {
	if strings.TrimSpace(workerID) == "" {
		workerID = uuid.NewString()
	}
	if limit <= 0 {
		limit = 10
	}

	events, err := s.repo.ClaimOutboxEvents(ctx, workerID, s.now(), limit)
	if err != nil {
		return 0, err
	}

	published := 0
	for _, event := range events {
		if ctx.Err() != nil {
			return published, ctx.Err()
		}

		if err := s.publishOutboxEvent(ctx, event); err != nil {
			nextAttemptAt := s.now().Add(outboxRetryDelay(event.Attempts + 1))
			releaseErr := s.repo.ReleaseOutboxEvent(ctx, event.ID, nextAttemptAt, err.Error())
			if releaseErr != nil {
				return published, releaseErr
			}
			log.Printf(
				"[payments-api] outbox publish failed event_id=%s order_id=%s type=%s attempt=%d err=%v",
				event.ID,
				event.OrderID,
				event.EventType,
				event.Attempts+1,
				err,
			)
			continue
		}

		if err := s.repo.MarkOutboxEventPublished(ctx, event.ID, s.now()); err != nil {
			return published, err
		}
		published++
	}

	return published, nil
}

func (s *PaymentsService) ReconcileOpenOrders(ctx context.Context, provider domain.Provider, olderThan time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 10
	}

	checkoutProvider, ok := s.providers[provider]
	if !ok || checkoutProvider == nil {
		return 0, nil
	}

	reconciler, ok := checkoutProvider.(OpenOrderReconciler)
	if !ok {
		return 0, nil
	}

	orders, err := s.repo.ListOpenOrdersForReconciliation(ctx, provider, s.now(), olderThan, limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for idx := range orders {
		order := orders[idx]
		input, err := reconciler.ReconcileOpenOrder(ctx, &order)
		if err != nil {
			log.Printf(
				"[payments-api] reconcile open order failed order_id=%s provider=%s err=%v",
				order.ID,
				order.Provider,
				err,
			)
			continue
		}
		if input == nil {
			continue
		}

		if _, err := s.ProcessWebhook(ctx, *input); err != nil {
			log.Printf(
				"[payments-api] reconcile open order process failed order_id=%s provider=%s err=%v",
				order.ID,
				order.Provider,
				err,
			)
			continue
		}
		processed++
	}

	return processed, nil
}

func (s *PaymentsService) ExpireStaleOpenOrders(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 10
	}

	now := s.now()
	orders, err := s.repo.ListExpiredOpenOrders(ctx, now, limit)
	if err != nil {
		return 0, err
	}

	expiredCount := 0
	for _, order := range orders {
		expired, err := s.repo.ExpireOrder(ctx, order.ID, "expired", now)
		if err != nil {
			log.Printf("[payments-api] expire order failed order_id=%s err=%v", order.ID, err)
			continue
		}
		expiredCount++

		if !shouldCancelPendingOnExpiration(expired) {
			continue
		}

		hasOtherOpen, err := s.repo.HasOtherOpenOrder(ctx, expired.UserID, expired.CourseID, expired.ID)
		if err != nil {
			return expiredCount, err
		}
		if hasOtherOpen {
			continue
		}

		if err := s.enrollments.CancelPendingEnrollment(ctx, expired.UserID, expired.CourseID); err != nil {
			log.Printf(
				"[payments-api] cancel pending enrollment after expiration failed order_id=%s user_id=%s course_id=%s err=%v",
				expired.ID,
				expired.UserID,
				expired.CourseID,
				err,
			)
		}
	}

	return expiredCount, nil
}

func (s *PaymentsService) publishOutboxEvent(ctx context.Context, event domain.PaymentOutboxEvent) error {
	switch event.EventType {
	case domain.OutboxEventPaymentPaid:
		var payload PaymentPaidEvent
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return err
		}
		return s.publisher.PublishPaymentPaid(ctx, payload)
	default:
		return nil
	}
}

func shouldCancelPendingOnExpiration(order *domain.Order) bool {
	if order == nil {
		return false
	}
	if order.ProviderPaymentID != nil {
		return false
	}

	providerStatus := strings.ToLower(strings.TrimSpace(derefString(order.ProviderStatus)))
	switch providerStatus {
	case "", "created", "cancelled", "canceled", "rejected", "failed", "expired", "superseded":
		return true
	default:
		return false
	}
}

func outboxRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	return time.Duration(1<<uint(attempt-1)) * time.Second
}

func (s *PaymentsService) QueueWebhook(ctx context.Context, input QueueWebhookInput) (*domain.PaymentWebhookJob, bool, error) {
	receivedAt := input.ReceivedAt.UTC()
	if receivedAt.IsZero() {
		receivedAt = s.now()
	}

	job := domain.PaymentWebhookJob{
		ID:                 uuid.NewString(),
		Provider:           input.Provider,
		DedupeKey:          strings.TrimSpace(input.DedupeKey),
		ResourceID:         strings.TrimSpace(input.ResourceID),
		RequestID:          normalizeOptionalString(input.RequestID),
		SignatureTimestamp: normalizeOptionalString(input.SignatureTimestamp),
		Topic:              normalizeOptionalString(input.Topic),
		Action:             normalizeOptionalString(input.Action),
		Payload:            strings.TrimSpace(input.Payload),
		Attempts:           0,
		ReceivedAt:         receivedAt,
		AvailableAt:        receivedAt,
	}

	return s.repo.EnqueueWebhookJob(ctx, job)
}

func (s *PaymentsService) ProcessWebhookJobs(ctx context.Context, workerID string, limit int) (int, error) {
	if strings.TrimSpace(workerID) == "" {
		workerID = uuid.NewString()
	}
	if limit <= 0 {
		limit = 10
	}

	jobs, err := s.repo.ClaimWebhookJobs(ctx, workerID, s.now(), limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, job := range jobs {
		if ctx.Err() != nil {
			return processed, ctx.Err()
		}

		jobProcessed, err := s.processClaimedWebhookJob(ctx, job)
		if err != nil {
			return processed, err
		}
		if jobProcessed {
			processed++
		}
	}

	return processed, nil
}

func (s *PaymentsService) ProcessWebhookJobByID(ctx context.Context, workerID, jobID string) (bool, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return false, ErrWebhookPayloadInvalid
	}
	if strings.TrimSpace(workerID) == "" {
		workerID = uuid.NewString()
	}

	job, err := s.repo.ClaimWebhookJobByID(ctx, jobID, workerID, s.now())
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			return false, nil
		}
		return false, err
	}

	return s.processClaimedWebhookJob(ctx, *job)
}

func (s *PaymentsService) ProcessWebhookJobDirect(ctx context.Context, job domain.PaymentWebhookJob) (bool, error) {
	return s.processClaimedWebhookJob(ctx, job)
}

func (s *PaymentsService) processClaimedWebhookJob(ctx context.Context, job domain.PaymentWebhookJob) (bool, error) {
	checkoutProvider, ok := s.providers[job.Provider]
	if !ok || checkoutProvider == nil {
		if err := s.repo.ReleaseWebhookJob(ctx, job.ID, s.now().Add(webhookJobRetryDelay(job.Attempts+1)), ErrProviderUnsupported.Error()); err != nil {
			return false, err
		}
		return false, nil
	}

	reconciler, ok := checkoutProvider.(PaymentReconciler)
	if !ok {
		if err := s.repo.ReleaseWebhookJob(ctx, job.ID, s.now().Add(webhookJobRetryDelay(job.Attempts+1)), ErrProviderRequestFailed.Error()); err != nil {
			return false, err
		}
		return false, nil
	}

	reconciled, err := reconciler.ReconcilePayment(ctx, ReconcilePaymentInput{
		PaymentID: job.ResourceID,
	})
	if err != nil {
		if err := s.repo.ReleaseWebhookJob(ctx, job.ID, s.now().Add(webhookJobRetryDelay(job.Attempts+1)), err.Error()); err != nil {
			return false, err
		}
		log.Printf(
			"[payments-api] webhook job reconcile failed job_id=%s provider=%s resource_id=%s attempt=%d err=%v",
			job.ID,
			job.Provider,
			job.ResourceID,
			job.Attempts+1,
			err,
		)
		return false, nil
	}

	reconciled.RequestID = normalizeOptionalString(job.RequestID)
	reconciled.Topic = normalizeOptionalString(job.Topic)
	reconciled.Action = normalizeOptionalString(job.Action)
	reconciled.ResourceID = optionalTrimmedString(job.ResourceID)
	if strings.TrimSpace(reconciled.Payload) == "" {
		reconciled.Payload = strings.TrimSpace(job.Payload)
	}
	if reconciled.ReceivedAt.IsZero() {
		reconciled.ReceivedAt = job.ReceivedAt
	}

	if _, err := s.ProcessWebhook(ctx, *reconciled); err != nil {
		if errors.Is(err, ErrPaymentConflict) {
			if markErr := s.repo.MarkWebhookJobProcessed(ctx, job.ID, s.now()); markErr != nil {
				return false, markErr
			}
			log.Printf(
				"[payments-api] webhook job marked processed with recorded anomaly job_id=%s provider=%s resource_id=%s err=%v",
				job.ID,
				job.Provider,
				job.ResourceID,
				err,
			)
			return true, nil
		}
		if err := s.repo.ReleaseWebhookJob(ctx, job.ID, s.now().Add(webhookJobRetryDelay(job.Attempts+1)), err.Error()); err != nil {
			return false, err
		}
		log.Printf(
			"[payments-api] webhook job processing failed job_id=%s provider=%s resource_id=%s attempt=%d err=%v",
			job.ID,
			job.Provider,
			job.ResourceID,
			job.Attempts+1,
			err,
		)
		return false, nil
	}

	if err := s.repo.MarkWebhookJobProcessed(ctx, job.ID, s.now()); err != nil {
		return false, err
	}
	return true, nil
}

func hasConflictingProviderPayment(order *domain.Order, incoming *string) bool {
	if order == nil || incoming == nil {
		return false
	}

	existingID := strings.TrimSpace(derefString(order.ProviderPaymentID))
	incomingID := strings.TrimSpace(derefString(incoming))
	if existingID == "" || incomingID == "" {
		return false
	}

	return existingID != incomingID
}

func buildProviderPaymentConflictIssue(order *domain.Order, input ProcessWebhookInput, now time.Time) domain.PaymentOrderIssue {
	incomingPaymentID := normalizeOptionalString(input.ProviderPaymentID)
	details := strings.TrimSpace("existingProviderPaymentId=" + derefString(order.ProviderPaymentID) + ", incomingProviderPaymentId=" + derefString(incomingPaymentID) + ", eventKey=" + strings.TrimSpace(input.EventKey) + ", resourceId=" + derefString(input.ResourceID))
	return domain.PaymentOrderIssue{
		ID:                uuid.NewString(),
		OrderID:           order.ID,
		IssueKey:          "provider-payment-conflict:" + strings.TrimSpace(derefString(order.ProviderPaymentID)) + ":" + strings.TrimSpace(derefString(incomingPaymentID)),
		IssueType:         "provider_payment_conflict",
		ProviderPaymentID: incomingPaymentID,
		Details:           details,
		Status:            "open",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func optionalTrimmedString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func webhookJobRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	return time.Duration(1<<uint(attempt-1)) * 2 * time.Second
}
