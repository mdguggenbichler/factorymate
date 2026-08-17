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

	deliveryDM := modsSubDeliveryDM(data)

	switch sub {
	case "export":
		b.handleModsExport(ctx, s, i, externalID, deliveryDM)
	default:
		b.handleModsList(ctx, s, i, externalID, deliveryDM)
	}
}

func modsSubDeliveryDM(data discordgo.ApplicationCommandInteractionData) bool {
	if len(data.Options) == 0 {
		return false
	}
	sub := data.Options[0]
	for _, opt := range sub.Options {
		if opt.Name == "delivery" {
			return opt.BoolValue()
		}
	}
	return false
}

func (b *Bot) handleModsList(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, deliveryDM bool) {
	list, err := b.mods.List(ctx)
	if err != nil {
		respondEphemeral(s, i, "Could not load mod list.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods list", false, err.Error())
		return
	}

	content := formatModsListMessage(list)
	if deliveryDM {
		if err := sendUserDM(s, externalID, content); err != nil {
			respondEphemeral(s, i, "Could not send mod list DM. Check that DMs are enabled.")
			_ = LogBotCommand(ctx, b.db, externalID, "mods list", false, err.Error())
			return
		}
		respondEphemeral(s, i, "Mod list sent to your DMs.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods list", true, "dm")
		return
	}
	respondEphemeral(s, i, content)
	_ = LogBotCommand(ctx, b.db, externalID, "mods list", true, "ephemeral")
}

func (b *Bot) handleModsExport(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, deliveryDM bool) {
	data, filename, err := b.mods.GenerateSMMProfile(ctx)
	if err != nil {
		respondEphemeral(s, i, "Could not generate SMM profile. Try again later.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods export", false, err.Error())
		return
	}

	file := &discordgo.File{Name: filename, Reader: bytes.NewReader(data)}
	if deliveryDM {
		if err := sendUserDMWithFile(s, externalID, "SMM profile ready — import this file in Satisfactory Mod Manager.", file); err != nil {
			respondEphemeral(s, i, "Could not send SMM profile DM. Check that DMs are enabled.")
			_ = LogBotCommand(ctx, b.db, externalID, "mods export", false, err.Error())
			return
		}
		respondEphemeral(s, i, "SMM profile sent to your DMs.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods export", true, "dm:"+filename)
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "SMM profile ready — import this file in Satisfactory Mod Manager.",
			Files:   []*discordgo.File{file},
			Flags:   discordgo.MessageFlagsEphemeral,
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
	const maxContentLen = 1900

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

	prefixLen := len(strings.Join(lines, "\n")) + 2 // trailing newline before mods
	modLines := make([]string, 0, len(list.Mods))
	for _, m := range list.Mods {
		modLines = append(modLines, fmt.Sprintf("%s — %s", m.Name, m.Version))
	}

	footer := fmt.Sprintf("\n\nFull list: %s/mods", PublicURL())
	remaining := maxContentLen - prefixLen - len(footer)
	if remaining < 0 {
		remaining = 0
	}

	shown := 0
	for _, line := range modLines {
		extra := len(line) + 1
		if shown > 0 && len(strings.Join(lines, "\n"))+extra+len(footer) > maxContentLen {
			break
		}
		if len(line) > remaining && shown == 0 {
			line = truncateRunes(line, remaining)
		}
		lines = append(lines, line)
		shown++
		remaining -= extra
	}

	if shown < len(list.Mods) {
		lines = append(lines, fmt.Sprintf("… and %d more mods (see %s/mods)", len(list.Mods)-shown, PublicURL()))
	}
	lines = append(lines, footer)
	return strings.Join(lines, "\n")
}

func truncateRunes(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
