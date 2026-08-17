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
	if len(data.Options) > 0 && data.Options[0].Name == "set" {
		if !CanRunAdminCommand(perms, state, fmUser) {
			b.logAndDeny(ctx, s, i, externalID, "connection set", "forbidden")
			return
		}
		adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
		if err != nil || adminUser == nil {
			respondEphemeral(s, i, "Something went wrong.")
			return
		}
		b.handleConnectionSet(ctx, s, i, externalID, adminUser.ID, data.Options[0])
		return
	}

	if !CanRunCommand(perms, CommandGroupConnection, state) {
		b.logAndDeny(ctx, s, i, externalID, "connection", "forbidden")
		return
	}

	public := false
	for _, opt := range data.Options {
		if opt.Name == "public" {
			public = opt.BoolValue()
		}
	}

	if public {
		details, err := b.connection.Get(ctx)
		if err != nil || strings.TrimSpace(details.GameHost) == "" {
			respondEphemeral(s, i, "Connection details are not configured yet.")
			_ = LogBotCommand(ctx, b.db, externalID, "connection", false, "not configured")
			return
		}
		respondEphemeral(s, i, connection.FormatDetailsDM(details).Plain)
		_ = LogBotCommand(ctx, b.db, externalID, "connection", true, "ephemeral")
		return
	}

	if err := b.connection.SendToUser(ctx, externalID); err != nil {
		respondEphemeral(s, i, "Could not send connection details DM. Check that DMs are enabled.")
		_ = LogBotCommand(ctx, b.db, externalID, "connection", false, err.Error())
		return
	}
	respondEphemeral(s, i, "Connection details sent to your DMs.")
	_ = LogBotCommand(ctx, b.db, externalID, "connection", true, "dm")
}

func (b *Bot) handleConnectionSet(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, adminID int64, sub *discordgo.ApplicationCommandInteractionDataOption) {
	var host string
	var port int64
	var password, notes string
	hasPassword := false

	for _, opt := range sub.Options {
		switch opt.Name {
		case "host":
			host = opt.StringValue()
		case "port":
			port = opt.IntValue()
		case "password":
			password = opt.StringValue()
			hasPassword = true
		case "notes":
			notes = opt.StringValue()
		}
	}

	input := connection.UpdateInput{
		GameHost: &host,
		GamePort: func() *int { p := int(port); return &p }(),
		Notes:    &notes,
	}
	if hasPassword {
		input.GamePassword = &password
	}

	old, _ := b.connection.Get(ctx)
	details, err := b.connection.Set(ctx, input, adminID)
	if err != nil {
		respondEphemeral(s, i, "Failed to update connection details.")
		_ = LogBotCommand(ctx, b.db, externalID, "connection set", false, err.Error())
		return
	}

	fields := connection.ChangedFields(old, details, input)
	detail := "updated: " + strings.Join(fields, ", ")
	respondEphemeral(s, i, fmt.Sprintf("Connection details updated and broadcast to active players.\nUpdated: %s", strings.Join(fields, ", ")))
	_ = LogBotCommand(ctx, b.db, externalID, "connection set", true, detail)
}
