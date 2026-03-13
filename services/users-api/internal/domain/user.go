package domain

import "time"

type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
)

func (r Role) IsValid() bool {
	switch r {
	case RoleStudent, RoleTeacher, RoleAdmin:
		return true
	default:
		return false
	}
}

type AuditAction string

const (
	ActionViewSensitive AuditAction = "VIEW_SENSITIVE"
	ActionChangeRole    AuditAction = "CHANGE_ROLE"
)

type User struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	IsVerified      bool       `json:"isVerified"`
	EmailVerified   bool       `json:"-"`
	EmailVerifiedAt *time.Time `json:"-"`
	Role            Role       `json:"role"`
	Phone           string     `json:"phone,omitempty"`
	DNI             string     `json:"dni,omitempty"`
	Address         string     `json:"address,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type EmailVerificationToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	UsedAt    *time.Time
}

type PasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	UsedAt    *time.Time
}

type AuditLog struct {
	ID           string
	AdminID      string
	Action       AuditAction
	TargetUserID string
	Timestamp    time.Time
	RequestID    string
	IP           string
}

type AuditMeta struct {
	RequestID string
	IP        string
}
