package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *Service) UserCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (s *Service) CreateUser(ctx context.Context, username, password string, role Role) (User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, created_at)
		VALUES (?, ?, ?, ?)`,
		username, hash, string(role), now,
	)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("last insert id: %w", err)
	}

	return User{ID: id, Username: username, Role: role}, nil
}

func (s *Service) Authenticate(ctx context.Context, username, password string) (User, error) {
	var user User
	var hash string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role FROM users WHERE username = ?`,
		username,
	).Scan(&user.ID, &user.Username, &hash, &user.Role)
	if err == sql.ErrNoRows {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	if !CheckPassword(hash, password) {
		return User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Service) GetUserByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, role FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Username, &user.Role)
	if err == sql.ErrNoRows {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	return user, nil
}

func (s *Service) UpdatePassword(ctx context.Context, userID int64, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}
