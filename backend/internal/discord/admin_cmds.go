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
		respondEphemeral(s, i, "Could not load server status.")
		_ = LogBotCommand(ctx, b.db, externalID, "status", false, err.Error())
		return
	}

	var onlineCount int
	if err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM player_state WHERE online = 1`).Scan(&onlineCount); err != nil {
		respondEphemeral(s, i, "Could not load player count.")
		_ = LogBotCommand(ctx, b.db, externalID, "status", false, err.Error())
		return
	}

	online := "offline"
	if serverOnline.Valid && serverOnline.Bool {
		online = "online"
	}
	msg := fmt.Sprintf("**%s** is **%s**.\nPlayers online: **%d**", serverName, online, onlineCount)
	respondEphemeral(s, i, msg)
	_ = LogBotCommand(ctx, b.db, externalID, "status", true, "")
}

func (b *Bot) handlePlayersCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string) {
	rows, err := b.db.QueryContext(ctx, `
		SELECT name FROM player_state WHERE online = 1 ORDER BY name`)
	if err != nil {
		respondEphemeral(s, i, "Could not load online players.")
		_ = LogBotCommand(ctx, b.db, externalID, "players", false, err.Error())
		return
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			respondEphemeral(s, i, "Could not load online players.")
			_ = LogBotCommand(ctx, b.db, externalID, "players", false, err.Error())
			return
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		respondEphemeral(s, i, "Could not load online players.")
		_ = LogBotCommand(ctx, b.db, externalID, "players", false, err.Error())
		return
	}

	var msg string
	if len(names) == 0 {
		msg = "No players are online right now."
	} else {
		msg = fmt.Sprintf("**Online players (%d):**\n%s", len(names), strings.Join(names, "\n"))
	}
	respondEphemeral(s, i, msg)
	_ = LogBotCommand(ctx, b.db, externalID, "players", true, "")
}

func (b *Bot) handleBroadcastCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		respondEphemeral(s, i, "Message is required.")
		_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, "empty message")
		return
	}
	if b.session == nil {
		respondEphemeral(s, i, "Discord bot is not connected.")
		_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, "bot offline")
		return
	}

	rows, err := b.db.QueryContext(ctx, `
		SELECT external_user_id FROM users
		WHERE status = 'active' AND external_user_id IS NOT NULL AND external_platform = 'discord'`)
	if err != nil {
		respondEphemeral(s, i, "Could not load recipients.")
		_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, err.Error())
		return
	}
	defer rows.Close()

	provider := notify.NewDiscordProvider(b.session)
	sent := 0
	for rows.Next() {
		var recipient string
		if err := rows.Scan(&recipient); err != nil {
			respondEphemeral(s, i, "Could not load recipients.")
			_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, err.Error())
			return
		}
		if err := provider.SendDirect(ctx, registration.PlatformDiscord, recipient, notify.RenderedMessage{Plain: message}); err == nil {
			sent++
		}
	}
	if err := rows.Err(); err != nil {
		respondEphemeral(s, i, "Could not load recipients.")
		_ = LogBotCommand(ctx, b.db, externalID, "broadcast", false, err.Error())
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Broadcast sent to **%d** player(s).", sent))
	_ = LogBotCommand(ctx, b.db, externalID, "broadcast", true, fmt.Sprintf("sent=%d", sent))
}

func (b *Bot) handleSyncRolesCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string) {
	guildID, err := EffectiveGuildID(ctx, b.db)
	if err != nil {
		respondEphemeral(s, i, "Could not load guild settings.")
		_ = LogBotCommand(ctx, b.db, externalID, "sync-roles", false, err.Error())
		return
	}
	updated, err := SyncAllLinkedRoles(ctx, b.db, b.session, guildID)
	if err != nil {
		respondEphemeral(s, i, "Role sync failed.")
		_ = LogBotCommand(ctx, b.db, externalID, "sync-roles", false, err.Error())
		return
	}
	respondEphemeral(s, i, fmt.Sprintf("Role sync complete. Updated **%d** user(s).", updated))
	_ = LogBotCommand(ctx, b.db, externalID, "sync-roles", true, fmt.Sprintf("updated=%d", updated))
}

func (b *Bot) handleNotificationsCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, userID int64, data discordgo.ApplicationCommandInteractionData) {
	svc := notifications.NewService(b.db)
	if len(data.Options) == 0 {
		b.showNotificationPrefs(ctx, s, i, externalID, svc, userID)
		return
	}

	sub := data.Options[0]
	switch sub.Name {
	case "view":
		b.showNotificationPrefs(ctx, s, i, externalID, svc, userID)
	case "category":
		b.setNotificationCategory(ctx, s, i, externalID, svc, userID, sub)
	case "personal":
		b.setNotificationPersonal(ctx, s, i, externalID, svc, userID, sub)
	default:
		respondEphemeral(s, i, "Unknown subcommand.")
	}
}

func (b *Bot) showNotificationPrefs(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, svc *notifications.Service, userID int64) {
	prefs, err := svc.GetUserPrefs(ctx, userID)
	if err != nil {
		respondEphemeral(s, i, "Could not load notification preferences.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications", false, err.Error())
		return
	}

	lines := []string{"**DM notification preferences**", ""}
	for _, category := range notifications.AllCategories {
		state := "off"
		if prefs.Categories[category] {
			state = "on"
		}
		lines = append(lines, fmt.Sprintf("- `%s`: **%s**", category, state))
	}
	personal := "off"
	if prefs.DMPlayerPersonal {
		personal = "on"
	}
	lines = append(lines, "", fmt.Sprintf("Personal player events: **%s**", personal))
	lines = append(lines, "", "Use `/notifications category <name> <on|off>` or `/notifications personal <on|off>` to change settings.")
	respondEphemeral(s, i, strings.Join(lines, "\n"))
	_ = LogBotCommand(ctx, b.db, externalID, "notifications", true, "view")
}

func (b *Bot) setNotificationCategory(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, svc *notifications.Service, userID int64, sub *discordgo.ApplicationCommandInteractionDataOption) {
	var category, enabled string
	for _, opt := range sub.Options {
		switch opt.Name {
		case "name":
			category = opt.StringValue()
		case "enabled":
			enabled = opt.StringValue()
		}
	}
	if !isValidCategory(category) {
		respondEphemeral(s, i, "Unknown category.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications category", false, category)
		return
	}
	on := enabled == "on"
	prefs, err := svc.GetUserPrefs(ctx, userID)
	if err != nil {
		respondEphemeral(s, i, "Could not update preferences.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications category", false, err.Error())
		return
	}
	prefs.Categories[category] = on
	if _, err := svc.SetUserPrefs(ctx, userID, prefs); err != nil {
		respondEphemeral(s, i, "Could not update preferences.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications category", false, err.Error())
		return
	}
	respondEphemeral(s, i, fmt.Sprintf("`%s` DM notifications are now **%s**.", category, enabled))
	_ = LogBotCommand(ctx, b.db, externalID, "notifications category", true, category+":"+enabled)
}

func (b *Bot) setNotificationPersonal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, svc *notifications.Service, userID int64, sub *discordgo.ApplicationCommandInteractionDataOption) {
	enabled := ""
	for _, opt := range sub.Options {
		if opt.Name == "enabled" {
			enabled = opt.StringValue()
		}
	}
	prefs, err := svc.GetUserPrefs(ctx, userID)
	if err != nil {
		respondEphemeral(s, i, "Could not update preferences.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications personal", false, err.Error())
		return
	}
	prefs.DMPlayerPersonal = enabled == "on"
	if _, err := svc.SetUserPrefs(ctx, userID, prefs); err != nil {
		respondEphemeral(s, i, "Could not update preferences.")
		_ = LogBotCommand(ctx, b.db, externalID, "notifications personal", false, err.Error())
		return
	}
	respondEphemeral(s, i, fmt.Sprintf("Personal player event DMs are now **%s**.", enabled))
	_ = LogBotCommand(ctx, b.db, externalID, "notifications personal", true, enabled)
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
