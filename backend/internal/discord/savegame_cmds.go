package discord

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"factorymate/internal/savegame"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleSavegameCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData, externalID string, userID int64, perms memberPermissions, state LinkState) {
	if !CanRunCommand(perms, CommandGroupSavegame, state) {
		b.logAndDeny(ctx, s, i, externalID, "savegame", "forbidden")
		return
	}
	if b.savegame == nil {
		respondEphemeral(ctx, s, i, "Save download is not available.")
		_ = LogBotCommand(ctx, b.db, externalID, "savegame", false, "no service")
		return
	}

	deliveryDM := true
	for _, opt := range data.Options {
		if opt.Name == "delivery" {
			deliveryDM = opt.BoolValue()
		}
	}

	result, err := b.savegame.DownloadLatest(ctx, userID, savegame.ChannelDiscord)
	if err != nil {
		msg := "Could not download save. Try again later."
		switch {
		case errors.Is(err, savegame.ErrNotConfigured):
			msg = "Save download is not configured yet."
		case errors.Is(err, savegame.ErrRateLimited):
			msg = "Please wait a few minutes before downloading again."
		case errors.Is(err, savegame.ErrNoActiveSave):
			msg = "No save is available on the server right now."
		}
		respondEphemeral(ctx, s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "savegame", false, err.Error())
		return
	}

	file := &discordgo.File{Name: result.Filename, Reader: bytes.NewReader(result.Body)}
	if result.Size > savegame.DiscordMaxBytes {
		link := PublicURL()
		if link == "" {
			link = "/connection"
		} else {
			link = link + "/connection"
		}
		dmMsg := fmt.Sprintf("The save file is too large for Discord (%d MiB). Download it from the dashboard: %s", result.Size/(1024*1024), link)
		if deliveryDM {
			if err := sendUserDM(s, externalID, dmMsg); err != nil {
				respondEphemeral(ctx, s, i, "Could not send DM. Check that DMs are enabled.")
				_ = LogBotCommand(ctx, b.db, externalID, "savegame", false, err.Error())
				return
			}
			respondEphemeral(ctx, s, i, "Save is too large for Discord — link sent to your DMs.")
			_ = LogBotCommand(ctx, b.db, externalID, "savegame", true, "link_dm")
			return
		}
		respondEphemeral(ctx, s, i, dmMsg)
		_ = LogBotCommand(ctx, b.db, externalID, "savegame", true, "link_ephemeral")
		return
	}

	if deliveryDM {
		if err := sendUserDMWithFile(s, externalID, "Savegame ready — import this file in Satisfactory or Satisfactory Mod Manager.", file); err != nil {
			respondEphemeral(ctx, s, i, "Could not send save DM. Check that DMs are enabled.")
			_ = LogBotCommand(ctx, b.db, externalID, "savegame", false, err.Error())
			return
		}
		respondEphemeral(ctx, s, i, "Save sent to your DMs.")
		_ = LogBotCommand(ctx, b.db, externalID, "savegame", true, "dm:"+result.Filename)
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Savegame ready — import this file in Satisfactory or Satisfactory Mod Manager.",
			Files:   []*discordgo.File{file},
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not attach save file.")
		_ = LogBotCommand(ctx, b.db, externalID, "savegame", false, err.Error())
		return
	}
	_ = LogBotCommand(ctx, b.db, externalID, "savegame", true, result.Filename)
}
