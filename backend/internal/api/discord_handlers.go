package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"factorymate/internal/discord"
)

type discordSettingsResponse struct {
	BotEnabled      bool            `json:"botEnabled"`
	BotConnected    bool            `json:"botConnected"`
	TokenConfigured bool            `json:"tokenConfigured"`
	GuildID         string          `json:"guildId"`
	RoleMappings    json.RawMessage `json:"roleMappings"`
	AutoApprove     bool            `json:"autoApprove"`
}

type updateDiscordSettingsRequest struct {
	BotEnabled   *bool           `json:"botEnabled"`
	GuildID      *string         `json:"guildId"`
	RoleMappings json.RawMessage `json:"roleMappings"`
	AutoApprove  *bool           `json:"autoApprove"`
}

func (h *Handler) GetDiscordSettings(w http.ResponseWriter, r *http.Request) {
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
		BotEnabled:      botEnabled,
		BotConnected:    h.discord != nil && h.discord.Connected(),
		TokenConfigured: strings.TrimSpace(os.Getenv("DISCORD_BOT_TOKEN")) != "",
		GuildID:         guildID,
		RoleMappings:    json.RawMessage(roleMappings),
		AutoApprove:     autoApprove,
	})
}

func (h *Handler) UpdateDiscordSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req updateDiscordSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.BotEnabled != nil {
		if err := discord.SetSetting(ctx, h.db, discord.KeyBotEnabled, boolString(*req.BotEnabled)); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if req.GuildID != nil {
		if err := discord.SetSetting(ctx, h.db, discord.KeyGuildID, strings.TrimSpace(*req.GuildID)); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if len(req.RoleMappings) > 0 {
		if !json.Valid(req.RoleMappings) {
			writeError(w, r, http.StatusBadRequest, "roleMappings must be valid JSON")
			return
		}
		if err := discord.SetSetting(ctx, h.db, discord.KeyRoleMappingsJSON, string(req.RoleMappings)); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if req.AutoApprove != nil {
		if err := discord.SetSetting(ctx, h.db, discord.KeyAutoApprove, boolString(*req.AutoApprove)); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}

	h.GetDiscordSettings(w, r)
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
