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

	commands := slashCommands()
	if err := ValidateApplicationCommands(commands); err != nil {
		return fmt.Errorf("validate slash commands: %w", err)
	}

	if _, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, guildID, commands, discordgo.WithContext(ctx)); err != nil {
		return fmt.Errorf("register slash commands: %w", err)
	}
	return nil
}

func slashCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "register",
			Description: "Create your FactoryMate dashboard account",
		},
		{
			Name:        "register-user",
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
					Name:        "get",
					Description: "Get join details (DM by default)",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "public",
							Description: "Show join details in this channel instead of DM",
							Required:    false,
						},
					},
				},
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
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "list (default) or export",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "list", Value: "list"},
						{Name: "export", Value: "export"},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "delivery",
					Description: "Send to your DMs instead of ephemeral reply",
					Required:    false,
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
			Name:        "registrations",
			Description: "Manage pending registration queue (admin)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "Pending approval queue summary",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "approve",
					Description: "Approve a pending registration",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Discord user", Required: false},
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "id", Description: "FactoryMate user id", Required: false},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "reject",
					Description: "Reject a pending registration",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Discord user", Required: false},
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "id", Description: "FactoryMate user id", Required: false},
						{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Optional reason", Required: false},
					},
				},
			},
		},
		{
			Name:        "unlink",
			Description: "Remove Discord link from a FactoryMate account (admin)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "Discord user to unlink",
					Required:    true,
				},
			},
		},
		{
			Name:        "password-reset",
			Description: "Reset a user's dashboard password and DM a temporary password (admin)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "Discord user",
					Required:    true,
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
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "view (default), category, or personal",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "view", Value: "view"},
						{Name: "category", Value: "category"},
						{Name: "personal", Value: "personal"},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Category name (with action:category)",
					Required:    false,
					Choices:     categoryChoices(),
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "enabled",
					Description: "on or off (with action:category or action:personal)",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "on", Value: "on"},
						{Name: "off", Value: "off"},
					},
				},
			},
		},
	}
}

func categoryChoices() []*discordgo.ApplicationCommandOptionChoice {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(notifications.AllCategories))
	for _, category := range notifications.AllCategories {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: category, Value: category})
	}
	return choices
}
