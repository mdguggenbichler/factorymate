package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultInviteTTL = 7 * 24 * time.Hour

type InviteStatus string

const (
	InviteStatusPending  InviteStatus = "pending"
	InviteStatusAccepted InviteStatus = "accepted"
	InviteStatusExpired  InviteStatus = "expired"
	InviteStatusRevoked  InviteStatus = "revoked"
)

type Invite struct {
	ID               int64        `json:"id"`
	Token            string       `json:"token"`
	Role             Role         `json:"role"`
	CreatedBy        int64        `json:"createdBy"`
	CreatedAt        time.Time    `json:"createdAt"`
	ExpiresAt        time.Time    `json:"expiresAt"`
	AcceptedAt       *time.Time   `json:"acceptedAt,omitempty"`
	AcceptedByUserID *int64       `json:"acceptedByUserId,omitempty"`
	RevokedAt        *time.Time   `json:"revokedAt,omitempty"`
	Status           InviteStatus `json:"status"`
	AcceptedUsername *string      `json:"acceptedUsername,omitempty"`
}

var (
	ErrInviteNotFound    = errors.New("invite not found")
	ErrInviteNotPending  = errors.New("invite is not pending")
	ErrInviteExpired     = errors.New("invite expired")
	ErrDuplicateUsername = errors.New("username already exists")
)

func (s *Service) CreateInvite(ctx context.Context, createdBy int64, role Role, ttl time.Duration) (Invite, error) {
	if ttl <= 0 {
		ttl = DefaultInviteTTL
	}
	token, err := newToken()
	if err != nil {
		return Invite{}, err
	}

	now := time.Now().UTC()
	expires := now.Add(ttl)

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO invites (token, role, created_by, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`,
		token, string(role), createdBy,
		now.Format(time.RFC3339), expires.Format(time.RFC3339),
	)
	if err != nil {
		return Invite{}, fmt.Errorf("insert invite: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Invite{}, fmt.Errorf("last insert id: %w", err)
	}

	return Invite{
		ID:        id,
		Token:     token,
		Role:      role,
		CreatedBy: createdBy,
		CreatedAt: now,
		ExpiresAt: expires,
		Status:    InviteStatusPending,
	}, nil
}

func (s *Service) GetInviteByToken(ctx context.Context, token string) (Invite, error) {
	return s.scanInvite(ctx, `
		SELECT i.id, i.token, i.role, i.created_by, i.created_at, i.expires_at,
			i.accepted_at, i.accepted_by_user_id, i.revoked_at, u.username
		FROM invites i
		LEFT JOIN users u ON u.id = i.accepted_by_user_id
		WHERE i.token = ?`, token)
}

func (s *Service) GetInviteByID(ctx context.Context, id int64) (Invite, error) {
	return s.scanInvite(ctx, `
		SELECT i.id, i.token, i.role, i.created_by, i.created_at, i.expires_at,
			i.accepted_at, i.accepted_by_user_id, i.revoked_at, u.username
		FROM invites i
		LEFT JOIN users u ON u.id = i.accepted_by_user_id
		WHERE i.id = ?`, id)
}

func (s *Service) ListInvites(ctx context.Context) ([]Invite, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.token, i.role, i.created_by, i.created_at, i.expires_at,
			i.accepted_at, i.accepted_by_user_id, i.revoked_at, u.username
		FROM invites i
		LEFT JOIN users u ON u.id = i.accepted_by_user_id
		ORDER BY i.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query invites: %w", err)
	}
	defer rows.Close()

	invites := make([]Invite, 0)
	for rows.Next() {
		inv, err := scanInviteRow(rows)
		if err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}

func (s *Service) RevokeInvite(ctx context.Context, id int64) error {
	inv, err := s.GetInviteByID(ctx, id)
	if err != nil {
		return err
	}
	if deriveInviteStatus(inv) != InviteStatusPending {
		return ErrInviteNotPending
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE invites SET revoked_at = ? WHERE id = ? AND accepted_at IS NULL AND revoked_at IS NULL`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("revoke invite: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrInviteNotFound
	}
	return nil
}

func (s *Service) AcceptInvite(ctx context.Context, token, username, password string) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return User{}, fmt.Errorf("username and password are required")
	}

	inv, err := s.GetInviteByToken(ctx, token)
	if err != nil {
		return User{}, err
	}
	status := deriveInviteStatus(inv)
	switch status {
	case InviteStatusExpired:
		return User{}, ErrInviteExpired
	case InviteStatusRevoked, InviteStatusAccepted:
		return User{}, ErrInviteNotPending
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, created_at)
		VALUES (?, ?, ?, ?)`,
		username, hash, string(inv.Role), now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrDuplicateUsername
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("last insert id: %w", err)
	}

	acceptRes, err := tx.ExecContext(ctx, `
		UPDATE invites SET accepted_at = ?, accepted_by_user_id = ?
		WHERE id = ? AND accepted_at IS NULL AND revoked_at IS NULL`,
		now, userID, inv.ID,
	)
	if err != nil {
		return User{}, fmt.Errorf("update invite: %w", err)
	}
	n, _ := acceptRes.RowsAffected()
	if n == 0 {
		return User{}, ErrInviteNotPending
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit tx: %w", err)
	}

	return s.GetUserByID(ctx, userID)
}

func (s *Service) AdminCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE role = ?`, string(RoleAdmin),
	).Scan(&count)
	return count, err
}

func (s *Service) scanInvite(ctx context.Context, query string, arg any) (Invite, error) {
	row := s.db.QueryRowContext(ctx, query, arg)
	inv, err := scanInviteRow(row)
	if err == sql.ErrNoRows {
		return Invite{}, ErrInviteNotFound
	}
	return inv, err
}

type inviteScanner interface {
	Scan(dest ...any) error
}

func scanInviteRow(row inviteScanner) (Invite, error) {
	var inv Invite
	var role, createdAt, expiresAt string
	var acceptedAt, revokedAt sql.NullString
	var acceptedByUserID sql.NullInt64
	var acceptedUsername sql.NullString

	if err := row.Scan(
		&inv.ID, &inv.Token, &role, &inv.CreatedBy, &createdAt, &expiresAt,
		&acceptedAt, &acceptedByUserID, &revokedAt, &acceptedUsername,
	); err != nil {
		return Invite{}, err
	}

	inv.Role = Role(role)
	var err error
	inv.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return Invite{}, fmt.Errorf("parse created_at: %w", err)
	}
	inv.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return Invite{}, fmt.Errorf("parse expires_at: %w", err)
	}
	if acceptedAt.Valid {
		t, err := time.Parse(time.RFC3339, acceptedAt.String)
		if err != nil {
			return Invite{}, fmt.Errorf("parse accepted_at: %w", err)
		}
		inv.AcceptedAt = &t
	}
	if revokedAt.Valid {
		t, err := time.Parse(time.RFC3339, revokedAt.String)
		if err != nil {
			return Invite{}, fmt.Errorf("parse revoked_at: %w", err)
		}
		inv.RevokedAt = &t
	}
	if acceptedByUserID.Valid {
		id := acceptedByUserID.Int64
		inv.AcceptedByUserID = &id
	}
	if acceptedUsername.Valid {
		name := acceptedUsername.String
		inv.AcceptedUsername = &name
	}
	inv.Status = deriveInviteStatus(inv)
	return inv, nil
}

func deriveInviteStatus(inv Invite) InviteStatus {
	if inv.AcceptedAt != nil {
		return InviteStatusAccepted
	}
	if inv.RevokedAt != nil {
		return InviteStatusRevoked
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return InviteStatusExpired
	}
	return InviteStatusPending
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
