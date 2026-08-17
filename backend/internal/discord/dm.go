package discord

import (
	"context"
	"fmt"
	"strings"

	"factorymate/internal/connection"
	"factorymate/internal/notify"
	"factorymate/internal/registration"
)

// SendWelcomeDM sends post-approval welcome with connection details when configured.
func (b *Bot) SendWelcomeDM(ctx context.Context, externalUserID, username string) {
	if b.session == nil {
		return
	}
	enabled, err := BotEnabled(ctx, b.db)
	if err != nil || !enabled {
		return
	}
	provider := notify.NewDiscordProvider(b.session)

	welcome := formatWelcomeApprovedDM(username)
	_ = provider.SendDirect(ctx, registration.PlatformDiscord, externalUserID, notify.RenderedMessage{Plain: welcome})

	if b.connection != nil {
		details, err := b.connection.Get(ctx)
		if err == nil && strings.TrimSpace(details.GameHost) != "" {
			connMsg := connection.FormatDetailsDM(details)
			_ = provider.SendDirect(ctx, registration.PlatformDiscord, externalUserID, connMsg)
		}
	}

	hint := "Use /mods for the full mod list and /mods export to download an SMM profile to import before joining."
	_ = provider.SendDirect(ctx, registration.PlatformDiscord, externalUserID, notify.RenderedMessage{Plain: hint})
}

// SendRegistrationDeclinedDM notifies a registrant their request was rejected.
func (b *Bot) SendRegistrationDeclinedDM(ctx context.Context, externalUserID, comment string) {
	if b.session == nil {
		return
	}
	enabled, err := BotEnabled(ctx, b.db)
	if err != nil || !enabled {
		return
	}
	provider := notify.NewDiscordProvider(b.session)
	_ = provider.SendDirect(ctx, registration.PlatformDiscord, externalUserID, notify.RenderedMessage{
		Plain: formatRegistrationDeclinedDM(comment),
	})
}

// SendConnectionDM sends current connection details to a user.
func (b *Bot) SendConnectionDM(ctx context.Context, externalUserID string) error {
	if b.connection == nil {
		return fmt.Errorf("connection service unavailable")
	}
	return b.connection.SendToUser(ctx, externalUserID)
}
