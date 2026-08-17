package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

func (s *Service) CreateSession(ctx context.Context, userID int64) (Session, error) {
	id, err := newSessionID()
	if err != nil {
		return Session{}, err
	}

	now := time.Now().UTC()
	expires := now.Add(time.Duration(SessionMaxAge) * time.Second)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)`,
		id, userID,
		now.Format(time.RFC3339),
		expires.Format(time.RFC3339),
	)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}

	return Session{
		ID:        id,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: expires,
	}, nil
}

func (s *Service) GetSession(ctx context.Context, sessionID string) (Session, error) {
	var sess Session
	var createdAt, expiresAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, created_at, expires_at FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&sess.ID, &sess.UserID, &createdAt, &expiresAt)
	if err == sql.ErrNoRows {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("query session: %w", err)
	}

	sess.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return Session{}, fmt.Errorf("parse created_at: %w", err)
	}
	sess.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return Session{}, fmt.Errorf("parse expires_at: %w", err)
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.DeleteSession(ctx, sessionID)
		return Session{}, ErrSessionNotFound
	}
	return sess, nil
}

func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Service) SessionFromRequest(ctx context.Context, r *http.Request) (Session, User, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return Session{}, User{}, ErrSessionNotFound
	}
	if cookie.Value == "" {
		return Session{}, User{}, ErrSessionNotFound
	}

	sess, err := s.GetSession(ctx, cookie.Value)
	if err != nil {
		return Session{}, User{}, err
	}

	user, err := s.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return Session{}, User{}, err
	}
	return sess, user, nil
}

func SetSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   SessionMaxAge,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

func newSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
