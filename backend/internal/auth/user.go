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
	var playerID, playerName sql.NullString
	var createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.role, u.created_at, u.player_id, p.name
		FROM users u
		LEFT JOIN player_state p ON p.player_id = u.player_id
		WHERE u.id = ?`, id,
	).Scan(&user.ID, &user.Username, &user.Role, &createdAt, &playerID, &playerName)
	if err == sql.ErrNoRows {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	user.Status = "active"
	user.CreatedAt = createdAt
	if playerID.Valid {
		id := playerID.String
		user.PlayerID = &id
	}
	if playerName.Valid {
		name := playerName.String
		user.PlayerName = &name
	}
	return user, nil
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.role, u.created_at, u.player_id, p.name
		FROM users u
		LEFT JOIN player_state p ON p.player_id = u.player_id
		ORDER BY u.username`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		var playerID, playerName sql.NullString
		var createdAt string
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &createdAt, &playerID, &playerName); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		user.Status = "active"
		user.CreatedAt = createdAt
		if playerID.Valid {
			id := playerID.String
			user.PlayerID = &id
		}
		if playerName.Valid {
			name := playerName.String
			user.PlayerName = &name
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Service) UpdateUserRole(ctx context.Context, id int64, role Role) error {
	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		return err
	}
	if user.Role == RoleAdmin && role == RoleViewer {
		count, err := s.AdminCount(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastAdmin
		}
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, string(role), id)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *Service) UpdatePlayerID(ctx context.Context, id int64, playerID *string) error {
	if playerID != nil && *playerID != "" {
		var exists int
		err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM player_state WHERE player_id = ?`, *playerID,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check player: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("player not found")
		}
	}

	var val any
	if playerID == nil || *playerID == "" {
		val = nil
	} else {
		val = *playerID
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET player_id = ? WHERE id = ?`, val, id)
	if err != nil {
		return fmt.Errorf("update player_id: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		return err
	}
	if user.Role == RoleAdmin {
		count, err := s.AdminCount(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastAdmin
		}
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
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
