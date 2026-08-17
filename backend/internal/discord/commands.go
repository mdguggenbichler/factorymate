package discord

import (
	"context"
	"fmt"
	"log"

	"factorymate/internal/notifications"

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
			Name:        "connection",
			Description: "Get or set game join connection details",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "set",
					Description: "Update join details and broadcast to players (admin)",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionString, Name: "host", Description: "Game server host", Required: true},
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "port", Description: "Game server port", Required: true},
						{Type: discordgo.ApplicationCommandOptionString, Name: "password", Description: "Client join password", Required: false},
						{Type: discordgo.ApplicationCommandOptionString, Name: "notes", Description: "Optional notes", Required: false},
					},
				},
			},
		},
		{
			Name:        "mods",
			Description: "Server mod list and SMM profile export",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "Show full mod list (ephemeral)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "export",
					Description: "Download SMM profile file",
				},
			},
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
		{
			Name:        "status",
			Description: "Server online state and player count",
		},
		{
			Name:        "players",
			Description: "List players currently online",
		},
		{
			Name:        "broadcast",
			Description: "DM all registered players (admin)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message",
					Description: "Message to send",
					Required:    true,
				},
			},
		},
		{
			Name:        "sync-roles",
			Description: "Re-apply Discord role mappings (admin)",
		},
		{
			Name:        "notifications",
			Description: "View or update your DM notification preferences",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "view",
					Description: "Show current DM preferences",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "category",
					Description: "Enable or disable a DM category",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "Category name",
							Required:    true,
							Choices: categoryChoices(),
						},
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
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "personal",
					Description: "Toggle personal player join/leave DMs",
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

func categoryChoices() []*discordgo.ApplicationCommandOptionChoice {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(notifications.AllCategories))
	for _, category := range notifications.AllCategories {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: category, Value: category})
	}
	return choices
}