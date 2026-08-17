package discord

import (
	"context"

	"factorymate/internal/notify"
	"factorymate/internal/registration"
)

// SendWelcomeDM sends the post-approval welcome message.
func (b *Bot) SendWelcomeDM(ctx context.Context, externalUserID, username string) {
	if b.session == nil {
		return
	}
	enabled, err := BotEnabled(ctx, b.db)
	if err != nil || !enabled {
		return
	}
	provider := notify.NewDiscordProvider(b.session)
	_ = provider.SendDirect(ctx, registration.PlatformDiscord, externalUserID, notify.RenderedMessage{
		Plain: formatWelcomeApprovedDM(username),
	})
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
