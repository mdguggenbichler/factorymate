package discord

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"factorymate/internal/notifications"
	"factorymate/internal/notify"
	"factorymate/internal/registration"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleStatusCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string) {
	var serverOnline sql.NullBool
	var serverName string
	err := b.db.QueryRowContext(ctx, `
		SELECT ss.server_online, a.server_name
		FROM app_settings a
		LEFT JOIN server_state ss ON ss.id = 1
		WHERE a.id = 1`,
	).Scan(&serverOnline, &serverName)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not load server status.")
		_ = LogBotCommand(ctx, b.db, externalID, "status", false, err.Error())
		return
	}

	var onlineCount int
	if err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM player_state WHERE online = 1`).Scan(&onlineCount); err != nil {
		respondEphemeral(ctx, s, i, "Could not load player count.")
		_ = LogBotCommand(ctx, b.db, externalID, "status", false, err.Error())
		return
	}

	online := "offline"
	if serverOnline.Valid && serverOnline.Bool {
		online = "online"
	}
	msg := fmt.Sprintf("**%s** is **%s**.\nPlayers online: **%d**", serverName, online, onlineCount)
	respondEphemeral(ctx, s, i, msg)
	_ = LogBotCommand(ctx, b.db, externalID, "status", true, "")
}

func (b *Bot) handlePlayersCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string) {
	rows, err := b.db.QueryContext(ctx, `
		SELECT name FROM player_state WHERE online = 1 ORDER BY name`)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not load online players.")
		_ = LogBotCommand(ctx, b.db, externalID, "players", false, err.Error())
		return
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			respondEphemeral(ctx, s, i, "Could not load online players.")
			_ = LogBotCommand(ctx, b.db, externalID, "players", false, err.Error())
			return
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		respondEphemeral(ctx, s, i, "Could not load online players.")
		_ = LogBotCommand(ctx, b.db, externalID, "players", false, err.Error())
		return
	}

	var msg string
	if len(names) == 0 {
		msg = "No players are online right now."
	} else {
		msg = fmt.Sprintf("**Online players (%d):**\n%s", len(names), strings.Join(names, "\n"))
	}
	respondEphemeral(ctx, s, i, msg)
	_ = LogBotCommand(ctx, b.db, externalID, "players", true, "")
}

func (b *Bot) handleBroadcastCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		respondEphemeral(ctx, s, i, "Message is required.")
		_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, "empty message")
		return
	}
	if b.Session() == nil {
		respondEphemeral(ctx, s, i, "Discord bot is not connected.")
		_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, "bot offline")
		return
	}

	rows, err := b.db.QueryContext(ctx, `
		SELECT external_user_id FROM users
		WHERE status = 'active' AND external_user_id IS NOT NULL AND external_platform = 'discord'`)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not load recipients.")
		_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, err.Error())
		return
	}

	recipients := make([]string, 0)
	for rows.Next() {
		var recipient string
		if err := rows.Scan(&recipient); err != nil {
			rows.Close()
			respondEphemeral(ctx, s, i, "Could not load recipients.")
			_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, err.Error())
			return
		}
		recipients = append(recipients, recipient)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		respondEphemeral(ctx, s, i, "Could not load recipients.")
		_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, err.Error())
		return
	}
	rows.Close()

	provider := notify.NewDiscordProvider(b.Session())
	sent := 0
	for _, recipient := range recipients {
		if err := provider.SendDirect(ctx, registration.PlatformDiscord, recipient, notify.RenderedMessage{Plain: message}); err == nil {
			sent++
		}
	}

	respondEphemeral(ctx, s, i, fmt.Sprintf("Broadcast sent to **%d** player(s).", sent))
	_ = LogBotCommand(ctx, b.db, externalID, "broadcast", true, fmt.Sprintf("sent=%d", sent))
}

func (b *Bot) handleSyncRolesCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string) {
	guildID, err := EffectiveGuildID(ctx, b.db)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not load guild settings.")
		_ = LogBotCommand(ctx, b.db, externalID, "sync-roles", false, err.Error())
		return
	}
	updated, err := SyncAllLinkedRoles(ctx, b.db, b.Session(), guildID)
	if err != nil {
		respondEphemeral(ctx, s, i, "Role sync failed.")
		_ = LogBotCommand(ctx, b.db, externalID, "sync-roles", false, err.Error())
		return
	}
	respondEphemeral(ctx, s, i, fmt.Sprintf("Role sync complete. Updated **%d** user(s).", updated))
	_ = LogBotCommand(ctx, b.db, externalID, "sync-roles", true, fmt.Sprintf("updated=%d", updated))
}

func (b *Bot) handleNotificationsCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, userID int64, data discordgo.ApplicationCommandInteractionData) {
	svc := notifications.NewService(b.db)
	action, category, enabled := parseNotificationsOptions(data)

	switch action {
	case "view":
		b.showNotificationPrefs(ctx, s, i, externalID, svc, userID)
	case "category":
		if category == "" || enabled == "" {
			respondEphemeral(ctx, s, i, "Usage: `/notifications action:category name:<category> enabled:<on|off>`")
			return
		}
		b.setNotificationCategory(ctx, s, i, externalID, svc, userID, category, enabled)
	case "personal":
		if enabled == "" {
			respondEphemeral(ctx, s, i, "Usage: `/notifications action:personal enabled:<on|off>`")
			return
		}
		b.setNotificationPersonal(ctx, s, i, externalID, svc, userID, enabled)
	default:
		respondEphemeral(ctx, s, i, "Unknown action.")
	}
}

func parseNotificationsOptions(data discordgo.ApplicationCommandInteractionData) (action, category, enabled string) {
	action = "view"
	for _, opt := range data.Options {
		switch opt.Name {
		case "action":
			if v := strings.TrimSpace(opt.StringValue()); v != "" {
				action = v
			}
		case "name":
			category = opt.StringValue()
		case "enabled":
			enabled = opt.StringValue()
		}
	}
	return action, category, enabled
}

// ParseNotificationsOptionsForTest exposes notification option parsing for tests.
func ParseNotificationsOptionsForTest(data discordgo.ApplicationCommandInteractionData) (string, string, string) {
	return parseNotificationsOptions(data)
}

func (b *Bot) showNotificationPrefs(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, svc *notifications.Service, userID int64) {
	prefs, err := svc.GetUserPrefs(ctx, userID)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not load notification preferences.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications", false, err.Error())
		return
	}

	lines := []string{
		"**DM notification preferences**",
		"Guild channels and personal DMs are independent. Admins control channels; you control DMs.",
		"",
	}
	for _, category := range notifications.AllCategories {
		enabled, total := notifications.CategorySummary(prefs.Types, prefs.Catalog, category)
		lines = append(lines, fmt.Sprintf("- `%s`: **%s**", category, categoryStateLabel(enabled, total)))
	}
	personal := "off"
	if prefs.DMPlayerPersonal {
		personal = "on"
	}
	lines = append(lines, "", fmt.Sprintf("Personal player events: **%s**", personal))
	lines = append(lines, "", "Use `/notifications action:category name:<category> enabled:<on|off>` (sets every type in that category) or `/notifications action:personal enabled:<on|off>`.")
	respondNotifications(ctx, s, i, strings.Join(lines, "\n"))
	_ = LogBotCommand(ctx, b.db, externalID, "notifications", true, "view")
}

func (b *Bot) setNotificationCategory(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, svc *notifications.Service, userID int64, category, enabled string) {
	if !isValidCategory(category) {
		respondEphemeral(ctx, s, i, "Unknown category.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications category", false, category)
		return
	}
	on := enabled == "on"
	keys, err := svc.TypeKeysInCategory(ctx, category)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not update preferences.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications category", false, err.Error())
		return
	}
	types := make(map[string]bool, len(keys))
	for _, key := range keys {
		types[key] = on
	}
	prefs, err := svc.SetUserPrefs(ctx, userID, notifications.UserPrefsPatch{Types: types})
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not update preferences.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications category", false, err.Error())
		return
	}

	msg := fmt.Sprintf("`%s` DM notifications are now **%s** for all types in that category.", category, enabled)
	if names := notifications.CategoryOverlapTargetNames(prefs.Catalog, category); len(names) > 0 {
		msg += fmt.Sprintf("\nAlso posted to Discord channels (via: %s). Mute those channels if you only want DMs.", strings.Join(names, ", "))
	}
	respondNotifications(ctx, s, i, msg)
	_ = LogBotCommand(ctx, b.db, externalID, "notifications category", true, category+":"+enabled)
}

func (b *Bot) setNotificationPersonal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, svc *notifications.Service, userID int64, enabled string) {
	on := enabled == "on"
	if _, err := svc.SetUserPrefs(ctx, userID, notifications.UserPrefsPatch{DMPlayerPersonal: &on}); err != nil {
		respondEphemeral(ctx, s, i, "Could not update preferences.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications personal", false, err.Error())
		return
	}
	respondNotifications(ctx, s, i, fmt.Sprintf("Personal player event DMs are now **%s**.", enabled))
	_ = LogBotCommand(ctx, b.db, externalID, "notifications personal", true, enabled)
}

func categoryStateLabel(enabled, total int) string {
	if total == 0 || enabled == 0 {
		return "off"
	}
	if enabled == total {
		return "on"
	}
	return fmt.Sprintf("mixed (%d/%d)", enabled, total)
}

func isValidCategory(category string) bool {
	for _, c := range notifications.AllCategories {
		if c == category {
			return true
		}
	}
	return false
}

func (b *Bot) handleGuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m == nil || m.User == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	enabled, err := BotEnabled(ctx, b.db)
	if err != nil || !enabled {
		return
	}

	guildID, err := EffectiveGuildID(ctx, b.db)
	if err != nil || guildID == "" || m.GuildID != guildID {
		return
	}

	changed, newRole, err := SyncMemberRole(ctx, b.db, m.Roles, m.User.ID)
	if err != nil {
		return
	}
	if changed {
		_ = LogBotCommand(ctx, b.db, m.User.ID, "role_sync", true, string(newRole))
	}
}
