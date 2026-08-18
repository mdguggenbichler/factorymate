package discord

import (
	"context"
	"fmt"
	"log"

	"factorymate/internal/notify"
	"factorymate/internal/registration"

	"github.com/bwmarrin/discordgo"
)

// SendWelcomeDM sends post-approval welcome with connection details when configured.
func (b *Bot) SendWelcomeDM(ctx context.Context, externalUserID, username string) {
	if b.Session() == nil {
		return
	}
	enabled, err := BotEnabled(ctx, b.db)
	if err != nil || !enabled {
		return
	}
	provider := notify.NewDiscordProvider(b.Session())

	welcome := formatWelcomeApprovedDM(username)
	if err := provider.SendDirect(ctx, registration.PlatformDiscord, externalUserID, notify.RenderedMessage{Plain: welcome}); err != nil {
		log.Printf("discord bot: welcome dm: %v", err)
	}

	if b.connection != nil {
		if err := b.connection.SendToUser(ctx, externalUserID); err != nil {
			log.Printf("discord bot: welcome connection dm: %v", err)
		}
	}

	hint := "Use /mods for the full mod list and /mods export to download an SMM profile to import before joining."
	if err := provider.SendDirect(ctx, registration.PlatformDiscord, externalUserID, notify.RenderedMessage{Plain: hint}); err != nil {
		log.Printf("discord bot: welcome hint dm: %v", err)
	}
}

// SendRegistrationDeclinedDM notifies a registrant their request was rejected.
func (b *Bot) SendRegistrationDeclinedDM(ctx context.Context, externalUserID, comment string) {
	if b.Session() == nil {
		return
	}
	enabled, err := BotEnabled(ctx, b.db)
	if err != nil || !enabled {
		return
	}
	provider := notify.NewDiscordProvider(b.Session())
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

func sendUserDM(s *discordgo.Session, externalUserID, content string) error {
	ch, err := s.UserChannelCreate(externalUserID)
	if err != nil {
		return err
	}
	_, err = s.ChannelMessageSend(ch.ID, content)
	return err
}

func sendUserDMWithFile(s *discordgo.Session, externalUserID, content string, file *discordgo.File) error {
	ch, err := s.UserChannelCreate(externalUserID)
	if err != nil {
		return err
	}
	_, err = s.ChannelMessageSendComplex(ch.ID, &discordgo.MessageSend{
		Content: content,
		Files:   []*discordgo.File{file},
	})
	return err
}
