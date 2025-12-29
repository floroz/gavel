package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"` // Never return in JSON
	FullName     string    `json:"full_name" db:"full_name"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	AvatarURL    string    `json:"avatar_url" db:"avatar_url"`
	PhoneNumber  string    `json:"phone_number" db:"phone_number"`
	CountryCode  string    `json:"country_code" db:"country_code"`
}

type RefreshToken struct {
	TokenHash []byte    `db:"token_hash"`
	UserID    uuid.UUID `db:"user_id"`
	ExpiresAt time.Time `db:"expires_at"`
	Revoked   bool      `db:"revoked"`
	CreatedAt time.Time `db:"created_at"`
	UserAgent string    `db:"user_agent"`
	IPAddress string    `db:"ip_address"`
}

type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusPublished  OutboxStatus = "published"
	OutboxStatusFailed     OutboxStatus = "failed"
)

type OutboxEvent struct {
	ID          uuid.UUID    `db:"id"`
	EventType   string       `db:"event_type"`
	Payload     []byte       `db:"payload"`
	Status      OutboxStatus `db:"status"`
	CreatedAt   time.Time    `db:"created_at"`
	ProcessedAt *time.Time   `db:"processed_at"`
}
