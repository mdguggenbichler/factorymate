package discord

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"factorymate/internal/mods"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleModsCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData, externalID string, perms memberPermissions, state LinkState) {
	if !CanRunCommand(perms, CommandGroupMods, state) {
		b.logAndDeny(ctx, s, i, externalID, "mods", "forbidden")
		return
	}

	sub := "list"
	if len(data.Options) > 0 {
		sub = data.Options[0].Name
	}

	switch sub {
	case "export":
		b.handleModsExport(ctx, s, i, externalID)
	default:
		b.handleModsList(ctx, s, i, externalID)
	}
}

func (b *Bot) handleModsList(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string) {
	list, err := b.mods.List(ctx)
	if err != nil {
		respondEphemeral(s, i, "Could not load mod list.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods list", false, err.Error())
		return
	}

	content := formatModsListMessage(list)
	respondEphemeral(s, i, content)
	_ = LogBotCommand(ctx, b.db, externalID, "mods list", true, "")
}

func (b *Bot) handleModsExport(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string) {
	data, filename, err := b.mods.GenerateSMMProfile(ctx)
	if err != nil {
		respondEphemeral(s, i, "Could not generate SMM profile. Try again later.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods export", false, err.Error())
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "SMM profile ready — import this file in Satisfactory Mod Manager.",
			Files: []*discordgo.File{
				{Name: filename, Reader: bytes.NewReader(data)},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		respondEphemeral(s, i, "Could not attach SMM profile.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods export", false, err.Error())
		return
	}
	_ = LogBotCommand(ctx, b.db, externalID, "mods export", true, filename)
}

func formatModsListMessage(list mods.ListResponse) string {
	var cached string
	if list.CachedAt != "" {
		if t, err := time.Parse(time.RFC3339, list.CachedAt); err == nil {
			cached = t.UTC().Format("Jan 2, 2006 · 15:04 UTC")
		}
	}
	header := "📦 Server mods"
	if cached != "" {
		header = fmt.Sprintf("📦 Server mods (%s)", cached)
	}

	lines := []string{
		header,
		"",
		fmt.Sprintf("Game build: %s", list.GameBuild),
		fmt.Sprintf("SML: %s", list.SMLVersion),
		"",
		"Install ALL mods below at matching versions — or use /mods export for an SMM profile.",
		"",
	}
	for _, m := range list.Mods {
		lines = append(lines, fmt.Sprintf("%s — %s", m.Name, m.Version))
	}
	lines = append(lines, "", fmt.Sprintf("Full list: %s/mods", PublicURL()))
	return strings.Join(lines, "\n")
}
