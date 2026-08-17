package auth

import (
	"context"
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
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
	PlayerID *string `json:"playerId,omitempty"`
	PlayerName *string `json:"playerName,omitempty"`
	Status   string `json:"status,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
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
