package auth

import (
	"context"
	"encoding/json"
	"time"
)

const (
	CookieName      = "factorymate_session"
	SessionMaxAge   = 30 * 24 * 60 * 60 // 30 days in seconds
	CleanupInterval = time.Hour
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

const (
	StatusActive          = "active"
	StatusPendingApproval = "pending_approval"
)

type User struct {
	ID                 int64          `json:"id"`
	Username           string         `json:"username"`
	Role               Role           `json:"role"`
	PlayerID           *string        `json:"playerId,omitempty"`
	PlayerName         *string        `json:"playerName,omitempty"`
	PendingPlayerName  *string        `json:"pendingPlayerName,omitempty"`
	Status             string         `json:"status,omitempty"`
	RegistrationSource *string        `json:"registrationSource,omitempty"`
	CreatedAt          string         `json:"createdAt,omitempty"`
	External           ExternalFields `json:"-"`
}

// MarshalJSON flattens external identity fields for API responses.
func (u User) MarshalJSON() ([]byte, error) {
	type userJSON struct {
		ID                 int64   `json:"id"`
		Username           string  `json:"username"`
		Role               Role    `json:"role"`
		PlayerID           *string `json:"playerId,omitempty"`
		PlayerName         *string `json:"playerName,omitempty"`
		PendingPlayerName  *string `json:"pendingPlayerName,omitempty"`
		Status             string  `json:"status,omitempty"`
		RegistrationSource *string `json:"registrationSource,omitempty"`
		CreatedAt          string  `json:"createdAt,omitempty"`
		ExternalFields
	}
	return json.Marshal(userJSON{
		ID:                 u.ID,
		Username:           u.Username,
		Role:               u.Role,
		PlayerID:           u.PlayerID,
		PlayerName:         u.PlayerName,
		PendingPlayerName:  u.PendingPlayerName,
		Status:             u.Status,
		RegistrationSource: u.RegistrationSource,
		CreatedAt:          u.CreatedAt,
		ExternalFields:     u.External,
	})
}

type Session struct {
	ID        string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

type contextKey int

const userContextKey contextKey = iota

func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userContextKey).(User)
	return u, ok
}

func withUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}
