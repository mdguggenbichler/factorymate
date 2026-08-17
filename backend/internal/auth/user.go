package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UserUpdate fields are optional; omitted fields are not changed.
type UserUpdate struct {
	Role     *Role
	Password *string
	PlayerID **string
}

func (s *Service) UserCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (s *Service) CreateUser(ctx context.Context, username, password string, role Role) (User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
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
	var hash, status string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, status FROM users WHERE username = ?`,
		username,
	).Scan(&user.ID, &user.Username, &hash, &user.Role, &status)
	if err == sql.ErrNoRows {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	if !CheckPassword(hash, password) {
		return User{}, ErrInvalidCredentials
	}
	user.Status = status
	if user.Status == StatusPendingApproval {
		return User{}, ErrPendingApproval
	}
	return user, nil
}

func (s *Service) GetUserByID(ctx context.Context, id int64) (User, error) {
	var user User
	var playerID, playerName sql.NullString
	var createdAt, status string
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.role, u.created_at, u.status, u.player_id, p.name
		FROM users u
		LEFT JOIN player_state p ON p.player_id = u.player_id
		WHERE u.id = ?`, id,
	).Scan(&user.ID, &user.Username, &user.Role, &createdAt, &status, &playerID, &playerName)
	if err == sql.ErrNoRows {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	user.Status = status
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
		SELECT u.id, u.username, u.role, u.created_at, u.status, u.player_id, p.name
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
		var createdAt, status string
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &createdAt, &status, &playerID, &playerName); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		user.Status = status
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

func (s *Service) UpdateUser(ctx context.Context, id int64, update UserUpdate) (User, error) {
	if update.Password != nil && *update.Password != "" {
		if err := ValidatePassword(*update.Password); err != nil {
			return User{}, err
		}
	}
	if update.PlayerID != nil {
		pid := *update.PlayerID
		if pid != nil && *pid != "" {
			var exists int
			err := s.db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM player_state WHERE player_id = ?`, *pid,
			).Scan(&exists)
			if err != nil {
				return User{}, fmt.Errorf("check player: %w", err)
			}
			if exists == 0 {
				return User{}, fmt.Errorf("player not found")
			}
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentRole string
	err = tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, id).Scan(&currentRole)
	if err == sql.ErrNoRows {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}

	if update.Role != nil {
		newRole := *update.Role
		res, err := tx.ExecContext(ctx, `
			UPDATE users SET role = ? WHERE id = ? AND (
				role != 'admin' OR ? != 'viewer' OR
				(SELECT COUNT(*) FROM users WHERE role = 'admin') > 1
			)`, string(newRole), id, string(newRole))
		if err != nil {
			return User{}, fmt.Errorf("update role: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			if currentRole == string(RoleAdmin) && newRole == RoleViewer {
				return User{}, ErrLastAdmin
			}
			return User{}, ErrUserNotFound
		}
		currentRole = string(newRole)
	}

	if update.PlayerID != nil {
		pid := *update.PlayerID
		var val any
		if pid == nil || *pid == "" {
			val = nil
		} else {
			val = *pid
		}
		res, err := tx.ExecContext(ctx, `UPDATE users SET player_id = ? WHERE id = ?`, val, id)
		if err != nil {
			return User{}, fmt.Errorf("update player_id: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return User{}, ErrUserNotFound
		}
	}

	if update.Password != nil && *update.Password != "" {
		hash, err := HashPassword(*update.Password)
		if err != nil {
			return User{}, err
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE users SET password_hash = ? WHERE id = ?`, hash, id,
		)
		if err != nil {
			return User{}, fmt.Errorf("update password: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return User{}, ErrUserNotFound
		}
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit tx: %w", err)
	}

	return s.GetUserByID(ctx, id)
}

func (s *Service) UpdateUserRole(ctx context.Context, id int64, role Role) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET role = ? WHERE id = ? AND (
			role != 'admin' OR ? != 'viewer' OR
			(SELECT COUNT(*) FROM users WHERE role = 'admin') > 1
		)`, string(role), id, string(role))
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var currentRole string
		err := s.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, id).Scan(&currentRole)
		if err == sql.ErrNoRows {
			return ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("query user: %w", err)
		}
		if currentRole == string(RoleAdmin) && role == RoleViewer {
			return ErrLastAdmin
		}
		return ErrUserNotFound
	}
	return nil
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM users WHERE id = ? AND (
			role != 'admin' OR (SELECT COUNT(*) FROM users WHERE role = 'admin') > 1
		)`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var currentRole string
		err := s.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, id).Scan(&currentRole)
		if err == sql.ErrNoRows {
			return ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("query user: %w", err)
		}
		if currentRole == string(RoleAdmin) {
			return ErrLastAdmin
		}
		return ErrUserNotFound
	}
	return nil
}

func (s *Service) UpdatePassword(ctx context.Context, userID int64, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
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
