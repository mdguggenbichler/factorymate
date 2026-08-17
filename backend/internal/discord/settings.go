package discord

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	KeyBotEnabled          = "discord.bot_enabled"
	KeyGuildID             = "discord.guild_id"
	KeyRoleMappingsJSON    = "discord.role_mappings_json"
	KeyAutoApprove         = "registration.auto_approve"
	KeyConnectionDetails   = "connection.details_json"
	KeySMMProfileName      = "mods.smm_profile_name"
)

// GetSetting returns a KV app setting value or empty string when unset.
func GetSetting(ctx context.Context, db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_setting_kv WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

// SetSetting upserts a KV app setting.
func SetSetting(ctx context.Context, db *sql.DB, key, value string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// BotEnabled reports whether the discord bot kill switch is on.
func BotEnabled(ctx context.Context, db *sql.DB) (bool, error) {
	raw, err := GetSetting(ctx, db, KeyBotEnabled)
	if err != nil {
		return false, err
	}
	if raw == "" {
		return true, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return true, fmt.Errorf("parse %s: %w", KeyBotEnabled, err)
	}
	return enabled, nil
}

// EffectiveGuildID returns app_settings guild ID when set, else DISCORD_GUILD_ID env (§19 O13).
func EffectiveGuildID(ctx context.Context, db *sql.DB) (string, error) {
	guildID, err := GetSetting(ctx, db, KeyGuildID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(guildID) != "" {
		return strings.TrimSpace(guildID), nil
	}
	return strings.TrimSpace(os.Getenv("DISCORD_GUILD_ID")), nil
}
