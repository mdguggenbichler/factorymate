package discord

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) registerSlashCommands(ctx context.Context) error {
	if b.session == nil {
		return nil
	}
	guildID, err := EffectiveGuildID(ctx, b.db)
	if err != nil {
		return err
	}
	if guildID == "" {
		log.Printf("discord bot: guild_id unset — skipping slash command registration")
		return nil
	}

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "register",
			Description: "Create your FactoryMate dashboard account",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "user",
					Description: "Invite a Discord user to complete registration (admin)",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "user",
							Description: "Discord user to invite",
							Required:    true,
						},
					},
				},
			},
		},
		{
			Name:        "link",
			Description: "Attach Discord to an existing FactoryMate web account",
		},
		{
			Name:        "set-player",
			Description: "Update your in-game player name mapping",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Your in-game player name",
					Required:    true,
				},
			},
		},
		{
			Name:        "whoami",
			Description: "Show your FactoryMate link status",
		},
		{
			Name:        "help",
			Description: "FactoryMate quick start and command list",
		},
		{
			Name:        "registration",
			Description: "Registration settings (admin)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "auto-approve",
					Description: "Toggle automatic registration approval",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "enabled",
							Description: "on or off",
							Required:    true,
							Choices: []*discordgo.ApplicationCommandOptionChoice{
								{Name: "on", Value: "on"},
								{Name: "off", Value: "off"},
							},
						},
					},
				},
			},
		},
	}

	if _, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, guildID, commands, discordgo.WithContext(ctx)); err != nil {
		return fmt.Errorf("register slash commands: %w", err)
	}
	return nil
}