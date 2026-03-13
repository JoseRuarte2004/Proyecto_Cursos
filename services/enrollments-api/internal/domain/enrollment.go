package domain

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusCancelled Status = "cancelled"
	StatusRefunded  Status = "refunded"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusActive, StatusCancelled, StatusRefunded:
		return true
	default:
		return false
	}
}

type Enrollment struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	CourseID  string    `json:"courseId"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type PaymentActivationIssueStatus string

const (
	PaymentActivationIssueRetryable    PaymentActivationIssueStatus = "retryable"
	PaymentActivationIssueManualReview PaymentActivationIssueStatus = "manual_review"
	PaymentActivationIssueResolved     PaymentActivationIssueStatus = "resolved"
)

type PaymentActivationIssue struct {
	ID            string
	OrderID       string
	UserID        string
	CourseID      string
	ReasonCode    string
	Status        PaymentActivationIssueStatus
	LastError     string
	Attempts      int
	NextAttemptAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ResolvedAt    *time.Time
	LockedAt      *time.Time
	LockedBy      *string
}
