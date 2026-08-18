package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"factorymate/internal/notify"
)

func (h *Handler) validateDiscordTargetChannel(ctx context.Context, configJSON string) error {
	var cfg notify.DiscordConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("parse discord config: %w", err)
	}
	channelID := strings.TrimSpace(cfg.ChannelID)
	if channelID == "" {
		return fmt.Errorf("channel_id is required")
	}
	if h.discord == nil || !h.discord.Connected() {
		return nil
	}

	channels, err := h.discord.ListGuildTextChannels(ctx)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel.ID == channelID {
			return nil
		}
	}
	return fmt.Errorf("bot cannot see this channel — allow View Channel, Send Messages, and Embed Links for the bot role on that channel (and its parent category)")
}
