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
// Do not redeclare User fields here: shadowing breaks User.MarshalJSON flattening.
type MeUser struct {
	User
}

// ResolvedPlayerLink describes a user whose pending_player_name was auto-linked.
type ResolvedPlayerLink struct {
	ExternalUserID string
	PlayerName     string
}

// TryResolvePendingPlayers links users whose pending_player_name matches a server player.
func TryResolvePendingPlayers(ctx context.Context, db *sql.DB, playerID, playerName string) ([]ResolvedPlayerLink, error) {
	playerID = strings.TrimSpace(playerID)
	playerName = strings.TrimSpace(playerName)
	if playerID == "" || playerName == "" {
		return nil, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, external_user_id FROM users
		WHERE pending_player_name IS NOT NULL
			AND player_id IS NULL
			AND LOWER(pending_player_name) = LOWER(?)`, playerName,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending users: %w", err)
	}

	type pendingUser struct {
		id         int64
		externalID sql.NullString
	}
	pending := make([]pendingUser, 0)
	for rows.Next() {
		var u pendingUser
		if err := rows.Scan(&u.id, &u.externalID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan pending user: %w", err)
		}
		pending = append(pending, u)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var owner int64
	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE player_id = ?`, playerID).Scan(&owner)
	if err == nil {
		return nil, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check player owner: %w", err)
	}

	linked := make([]ResolvedPlayerLink, 0)
	for _, user := range pending {
		res, err := db.ExecContext(ctx, `UPDATE users SET player_id = ? WHERE id = ? AND player_id IS NULL`, playerID, user.id)
		if err != nil {
			return nil, fmt.Errorf("auto-link player: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			continue
		}
		link := ResolvedPlayerLink{PlayerName: playerName}
		if user.externalID.Valid {
			link.ExternalUserID = user.externalID.String
		}
		linked = append(linked, link)
	}
	return linked, nil
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
