package domain

import "time"

type Provider string

const (
	ProviderMercadoPago Provider = "mercadopago"
	ProviderStripe      Provider = "stripe"
)

func (p Provider) IsValid() bool {
	switch p {
	case ProviderMercadoPago, ProviderStripe:
		return true
	default:
		return false
	}
}

type Status string

const (
	StatusCreated  Status = "created"
	StatusPending  Status = "pending"
	StatusPaid     Status = "paid"
	StatusFailed   Status = "failed"
	StatusRefunded Status = "refunded"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusCreated, StatusPending, StatusPaid, StatusFailed, StatusRefunded:
		return true
	default:
		return false
	}
}

type Order struct {
	ID                   string
	UserID               string
	CourseID             string
	AmountCents          int64
	Currency             string
	Provider             Provider
	ProviderPaymentID    *string
	ProviderPreferenceID *string
	ExternalReference    *string
	CheckoutURL          *string
	ProviderStatus       *string
	Status               Status
	IdempotencyKey       string
	PaidAt               *time.Time
	FailedAt             *time.Time
	LastWebhookAt        *time.Time
	ExpiresAt            *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PaymentWebhookEvent struct {
	ID          string
	Provider    Provider
	EventKey    string
	RequestID   *string
	Topic       *string
	Action      *string
	ResourceID  *string
	OrderID     *string
	Payload     string
	CreatedAt   time.Time
	ProcessedAt *time.Time
}

type PaymentWebhookJob struct {
	ID                 string
	Provider           Provider
	DedupeKey          string
	ResourceID         string
	RequestID          *string
	SignatureTimestamp *string
	Topic              *string
	Action             *string
	Payload            string
	Attempts           int
	ReceivedAt         time.Time
	AvailableAt        time.Time
	ProcessedAt        *time.Time
	LockedAt           *time.Time
	LockedBy           *string
	LastError          *string
}

type PaymentOrderIssue struct {
	ID                string
	OrderID           string
	IssueKey          string
	IssueType         string
	ProviderPaymentID *string
	Details           string
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ResolvedAt        *time.Time
}

const OutboxEventPaymentPaid = "payment.paid"

type PaymentOutboxEvent struct {
	ID          string
	EventType   string
	OrderID     string
	Payload     string
	Attempts    int
	CreatedAt   time.Time
	AvailableAt time.Time
	PublishedAt *time.Time
	LockedAt    *time.Time
	LockedBy    *string
	LastError   *string
}
