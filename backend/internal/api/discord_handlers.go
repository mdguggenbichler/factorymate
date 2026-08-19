package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"factorymate/internal/auth"
	"factorymate/internal/discord"
)

type discordSettingsResponse struct {
	BotEnabled          bool            `json:"botEnabled"`
	BotConnected        bool            `json:"botConnected"`
	TokenConfigured     bool            `json:"tokenConfigured"`
	OAuthConfigured     bool            `json:"oauthConfigured"`
	GuildID             string          `json:"guildId"`
	RoleMappings        json.RawMessage `json:"roleMappings"`
	AutoApprove         bool            `json:"autoApprove"`
	CommandRegisterWarn string          `json:"commandRegisterWarning,omitempty"`
}

type updateDiscordSettingsRequest struct {
	BotEnabled   *bool           `json:"botEnabled"`
	GuildID      *string         `json:"guildId"`
	RoleMappings json.RawMessage `json:"roleMappings"`
	AutoApprove  *bool           `json:"autoApprove"`
}

func (h *Handler) GetDiscordSettings(w http.ResponseWriter, r *http.Request) {
	h.GetDiscordSettingsWithWriter(w, r, "")
}

func (h *Handler) GetDiscordSettingsWithWriter(w http.ResponseWriter, r *http.Request, cmdWarn string) {
	ctx := r.Context()

	botEnabled, err := discord.BotEnabled(ctx, h.db)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	guildID, err := discord.EffectiveGuildID(ctx, h.db)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	roleMappings, err := discord.GetSetting(ctx, h.db, discord.KeyRoleMappingsJSON)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if roleMappings == "" {
		roleMappings = "{}"
	}

	autoApproveRaw, err := discord.GetSetting(ctx, h.db, discord.KeyAutoApprove)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	autoApprove := true
	if autoApproveRaw != "" {
		autoApprove = autoApproveRaw == "true"
	}

	writeJSON(w, http.StatusOK, discordSettingsResponse{
		BotEnabled:          botEnabled,
		BotConnected:        h.discord != nil && h.discord.Connected(),
		TokenConfigured:     strings.TrimSpace(os.Getenv("DISCORD_BOT_TOKEN")) != "",
		OAuthConfigured:     auth.OAuthConfigured(),
		GuildID:             guildID,
		RoleMappings:        json.RawMessage(roleMappings),
		AutoApprove:         autoApprove,
		CommandRegisterWarn: cmdWarn,
	})
}

func (h *Handler) UpdateDiscordSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	oldGuildID, err := discord.EffectiveGuildID(ctx, h.db)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	var req updateDiscordSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.RoleMappings) > 0 && !json.Valid(req.RoleMappings) {
		writeError(w, r, http.StatusBadRequest, "roleMappings must be valid JSON")
		return
	}

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	upsert := func(key, value string) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO app_setting_kv (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			key, value,
		)
		return err
	}

	if req.BotEnabled != nil {
		if err := upsert(discord.KeyBotEnabled, boolString(*req.BotEnabled)); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if req.GuildID != nil {
		if err := upsert(discord.KeyGuildID, strings.TrimSpace(*req.GuildID)); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if len(req.RoleMappings) > 0 {
		if err := upsert(discord.KeyRoleMappingsJSON, string(req.RoleMappings)); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if req.AutoApprove != nil {
		if err := upsert(discord.KeyAutoApprove, boolString(*req.AutoApprove)); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	var cmdWarn string
	if h.discord != nil && h.discord.Connected() {
		newGuildID, err := discord.EffectiveGuildID(ctx, h.db)
		if err == nil && newGuildID != "" && newGuildID != oldGuildID {
			if oldGuildID != "" {
				if err := h.discord.ClearSlashCommands(ctx, oldGuildID); err != nil {
					log.Printf("discord: clear commands on old guild %s: %v", oldGuildID, err)
					cmdWarn = "Saved settings, but could not clear slash commands on the previous guild."
				}
			}
			if err := h.discord.RegisterSlashCommands(ctx); err != nil {
				log.Printf("discord: register commands on guild %s: %v", newGuildID, err)
				if cmdWarn == "" {
					cmdWarn = "Saved settings, but slash command registration failed — try saving again or restart the bot."
				}
			}
		}
	}

	h.GetDiscordSettingsWithWriter(w, r, cmdWarn)
}

func (h *Handler) ListDiscordChannels(w http.ResponseWriter, r *http.Request) {
	if h.discord == nil {
		writeError(w, r, http.StatusServiceUnavailable, "discord bot is not available")
		return
	}
	channels, err := h.discord.ListGuildTextChannels(r.Context())
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": channels})
}

func (h *Handler) GetDiscordInviteURL(w http.ResponseWriter, r *http.Request) {
	if h.discord == nil {
		writeError(w, r, http.StatusServiceUnavailable, "discord bot is not available")
		return
	}
	url, err := h.discord.InviteURL()
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"inviteUrl": url})
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
