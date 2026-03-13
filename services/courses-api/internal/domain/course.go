package domain

import "time"

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusDraft, StatusPublished:
		return true
	default:
		return false
	}
}

type Course struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	ImageURL    *string   `json:"imageUrl,omitempty"`
	Price       float64   `json:"price"`
	Currency    string    `json:"currency"`
	Capacity    int       `json:"capacity"`
	Status      Status    `json:"status"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
