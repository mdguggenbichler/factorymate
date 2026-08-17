package discord

import (
	"context"

	"factorymate/internal/connection"
	"factorymate/internal/mods"
	"factorymate/internal/notify"
)

// SendDirect implements connection.DirectMessenger via the live session.
func (b *Bot) SendDirect(ctx context.Context, platform, externalUserID string, msg notify.RenderedMessage) error {
	provider := notify.NewDiscordProvider(b.session)
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
