package discord

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"factorymate/internal/auth"

	"github.com/bwmarrin/discordgo"
)

// SyncMemberRole applies Discord role mappings to a linked FM user's role.
func SyncMemberRole(ctx context.Context, db *sql.DB, memberRoleIDs []string, externalUserID string) (bool, auth.Role, error) {
	externalUserID = strings.TrimSpace(externalUserID)
	if externalUserID == "" {
		return false, "", nil
	}

	var userID int64
	var currentRole string
	err := db.QueryRowContext(ctx, `
		SELECT id, role FROM users
		WHERE external_platform = 'discord' AND external_user_id = ? AND status = 'active'`,
		externalUserID,
	).Scan(&userID, &currentRole)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("query linked user: %w", err)
	}

	perms, err := ResolveMemberPermissions(ctx, db, memberRoleIDs)
	if err != nil {
		return false, "", err
	}
	targetRole := perms.FMRole
	if string(targetRole) == currentRole {
		return false, targetRole, nil
	}

	authSvc := auth.NewService(db)
	if err := authSvc.UpdateUserRole(ctx, userID, targetRole); err != nil {
		return false, "", fmt.Errorf("update fm role: %w", err)
	}
	return true, targetRole, nil
}

// SyncAllLinkedRoles re-applies Discord role mappings for every linked active user.
func SyncAllLinkedRoles(ctx context.Context, db *sql.DB, session *discordgo.Session, guildID string) (int, error) {
	if session == nil {
		return 0, fmt.Errorf("discord bot is not connected")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return 0, fmt.Errorf("discord guild_id is not configured")
	}

	rows, err := db.QueryContext(ctx, `
		SELECT external_user_id FROM users
		WHERE external_platform = 'discord'
			AND external_user_id IS NOT NULL
			AND status = 'active'`)
	if err != nil {
		return 0, fmt.Errorf("query linked users: %w", err)
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var externalUserID string
		if err := rows.Scan(&externalUserID); err != nil {
			return updated, fmt.Errorf("scan linked user: %w", err)
		}
		member, err := session.GuildMember(guildID, externalUserID, discordgo.WithContext(ctx))
		if err != nil {
			log.Printf("discord bot: sync roles: guild member %s: %v", externalUserID, err)
			continue
		}
		changed, _, err := SyncMemberRole(ctx, db, member.Roles, externalUserID)
		if err != nil {
			return updated, err
		}
		if changed {
			updated++
		}
	}
	return updated, rows.Err()
}
