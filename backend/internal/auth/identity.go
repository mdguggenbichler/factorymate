package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ExternalFields holds optional external identity columns on a user.
type ExternalFields struct {
	Platform    *string `json:"externalPlatform,omitempty"`
	UserID      *string `json:"externalUserId,omitempty"`
	Username    *string `json:"externalUsername,omitempty"`
	DisplayName *string `json:"externalDisplayName,omitempty"`
	LinkedAt    *string `json:"externalLinkedAt,omitempty"`
}

// MeUser extends User with fields returned by GET /api/auth/me (O15).
type MeUser struct {
	User
	PendingPlayerName *string `json:"pendingPlayerName,omitempty"`
	External          ExternalFields
}

// TryResolvePendingPlayers links users whose pending_player_name matches a server player.
func TryResolvePendingPlayers(ctx context.Context, db *sql.DB, playerID, playerName string) error {
	playerID = strings.TrimSpace(playerID)
	playerName = strings.TrimSpace(playerName)
	if playerID == "" || playerName == "" {
		return nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id FROM users
		WHERE pending_player_name IS NOT NULL
			AND player_id IS NULL
			AND LOWER(pending_player_name) = LOWER(?)`, playerName,
	)
	if err != nil {
		return fmt.Errorf("query pending users: %w", err)
	}

	userIDs := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			rows.Close()
			return fmt.Errorf("scan pending user: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var owner int64
	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE player_id = ?`, playerID).Scan(&owner)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("check player owner: %w", err)
	}

	for _, userID := range userIDs {
		if _, err := db.ExecContext(ctx, `UPDATE users SET player_id = ? WHERE id = ? AND player_id IS NULL`, playerID, userID); err != nil {
			return fmt.Errorf("auto-link player: %w", err)
		}
	}
	return nil
}

func loadExternalFields(platform, userID, username, displayName, linkedAt sql.NullString) ExternalFields {
	var ext ExternalFields
	if platform.Valid {
		v := platform.String
		ext.Platform = &v
	}
	if userID.Valid {
		v := userID.String
		ext.UserID = &v
	}
	if username.Valid {
		v := username.String
		ext.Username = &v
	}
	if displayName.Valid {
		v := displayName.String
		ext.DisplayName = &v
	}
	if linkedAt.Valid {
		v := linkedAt.String
		ext.LinkedAt = &v
	}
	return ext
}
