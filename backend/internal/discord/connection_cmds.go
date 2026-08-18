package discord

import (
	"context"
	"fmt"
	"strings"

	"factorymate/internal/auth"
	"factorymate/internal/connection"
	"factorymate/internal/registration"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleConnectionCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData, externalID string, perms memberPermissions, state LinkState, fmUser *auth.User) {
	if len(data.Options) == 0 {
		respondEphemeral(ctx, s, i, "Use /connection get or /connection set.")
		return
	}

	sub := data.Options[0]
	switch sub.Name {
	case "set":
		if !CanRunAdminCommand(perms, state, fmUser) {
			b.logAndDeny(ctx, s, i, externalID, "connection set", "forbidden")
			return
		}
		adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
		if err != nil || adminUser == nil {
			respondEphemeral(ctx, s, i, "Something went wrong.")
			return
		}
		b.handleConnectionSet(ctx, s, i, externalID, adminUser.ID, sub)
	case "get":
		if !CanRunCommand(perms, CommandGroupConnection, state) {
			b.logAndDeny(ctx, s, i, externalID, "connection get", "forbidden")
			return
		}
		b.handleConnectionGet(ctx, s, i, externalID)
	default:
		respondEphemeral(ctx, s, i, "Unknown subcommand.")
	}
}

func (b *Bot) handleConnectionGet(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string) {
	if err := b.connection.SendToUser(ctx, externalID); err != nil {
		respondEphemeral(ctx, s, i, "Could not send connection details DM. Check that DMs are enabled.")
		_ = LogBotCommand(ctx, b.db, externalID, "connection get", false, err.Error())
		return
	}
	respondEphemeral(ctx, s, i, "Connection details sent to your DMs.")
	_ = LogBotCommand(ctx, b.db, externalID, "connection get", true, "dm")
}

func (b *Bot) handleConnectionSet(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, adminID int64, sub *discordgo.ApplicationCommandInteractionDataOption) {
	var host string
	var port int64
	var password, notes string
	hasHost, hasPort, hasPassword, hasNotes := false, false, false, false

	for _, opt := range sub.Options {
		switch opt.Name {
		case "host":
			host = opt.StringValue()
			hasHost = true
		case "port":
			port = opt.IntValue()
			hasPort = true
		case "password":
			password = opt.StringValue()
			hasPassword = true
		case "notes":
			notes = opt.StringValue()
			hasNotes = true
		}
	}

	input := connection.UpdateInput{}
	if hasHost {
		input.GameHost = &host
	}
	if hasPort {
		p := int(port)
		input.GamePort = &p
	}
	if hasNotes {
		input.Notes = &notes
	}
	if hasPassword {
		input.GamePassword = &password
	}

	old, _ := b.connection.Get(ctx)
	details, err := b.connection.Set(ctx, input, adminID)
	if err != nil {
		respondEphemeral(ctx, s, i, "Failed to update connection details.")
		_ = LogBotCommand(ctx, b.db, externalID, "connection set", false, err.Error())
		return
	}

	fields := connection.ChangedFields(old, details, input)
	detail := "updated: " + strings.Join(fields, ", ")
	respondEphemeral(ctx, s, i, fmt.Sprintf("Connection details updated and broadcast to active players.\nUpdated: %s", strings.Join(fields, ", ")))
	_ = LogBotCommand(ctx, b.db, externalID, "connection set", true, detail)
}
