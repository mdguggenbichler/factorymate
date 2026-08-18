package discord

import (
	"context"
	"fmt"

	"factorymate/internal/connection"
	"factorymate/internal/mods"
	"factorymate/internal/notify"
)

// SendDirect implements connection.DirectMessenger via the live session.
func (b *Bot) SendDirect(ctx context.Context, platform, externalUserID string, msg notify.RenderedMessage) error {
	b.sessionMu.RLock()
	session := b.session
	b.sessionMu.RUnlock()
	if session == nil {
		return fmt.Errorf("discord bot is not connected")
	}
	provider := notify.NewDiscordProvider(session)
	return provider.SendDirect(ctx, platform, externalUserID, msg)
}

// SetConnection wires the connection service after the gateway session is available.
func (b *Bot) SetConnection(svc *connection.Service) {
	b.connection = svc
}

// SetMods wires the mods service (optional if set at construction).
func (b *Bot) SetMods(svc *mods.Service) {
	b.mods = svc
}
