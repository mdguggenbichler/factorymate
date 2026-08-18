package discord

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"factorymate/internal/mods"
	"factorymate/internal/notify"
	"factorymate/internal/registration"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleModsCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData, externalID string, perms memberPermissions, state LinkState) {
	if !CanRunCommand(perms, CommandGroupMods, state) {
		b.logAndDeny(ctx, s, i, externalID, "mods", "forbidden")
		return
	}

	action, deliveryDM := parseModsOptions(data)
	switch action {
	case "export":
		b.handleModsExport(ctx, s, i, externalID, deliveryDM)
	default:
		b.handleModsList(ctx, s, i, externalID, deliveryDM)
	}
}

func parseModsOptions(data discordgo.ApplicationCommandInteractionData) (action string, deliveryDM bool) {
	action = "list"
	for _, opt := range data.Options {
		switch opt.Name {
		case "action":
			if v := strings.TrimSpace(opt.StringValue()); v != "" {
				action = v
			}
		case "delivery":
			deliveryDM = opt.BoolValue()
		}
	}
	return action, deliveryDM
}

func (b *Bot) handleModsList(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, deliveryDM bool) {
	list, err := b.mods.List(ctx)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not load mod list.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods list", false, err.Error())
		return
	}

	embed := formatModsListEmbed(list)
	if deliveryDM {
		provider := notify.NewDiscordProvider(s)
		if err := provider.SendDirect(ctx, registration.PlatformDiscord, externalID, notify.RenderedMessage{Embed: embed}); err != nil {
			respondEphemeral(ctx, s, i, "Could not send mod list DM. Check that DMs are enabled.")
			_ = LogBotCommand(ctx, b.db, externalID, "mods list", false, err.Error())
			return
		}
		respondEphemeral(ctx, s, i, "Mod list sent to your DMs.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods list", true, "dm")
		return
	}
	respondEphemeralEmbed(ctx, s, i, embed)
	_ = LogBotCommand(ctx, b.db, externalID, "mods list", true, "ephemeral")
}

func (b *Bot) handleModsExport(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, deliveryDM bool) {
	data, filename, err := b.mods.GenerateSMMProfile(ctx)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not generate SMM profile. Try again later.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods export", false, err.Error())
		return
	}

	file := &discordgo.File{Name: filename, Reader: bytes.NewReader(data)}
	if deliveryDM {
		if err := sendUserDMWithFile(s, externalID, "SMM profile ready — import this file in Satisfactory Mod Manager.", file); err != nil {
			respondEphemeral(ctx, s, i, "Could not send SMM profile DM. Check that DMs are enabled.")
			_ = LogBotCommand(ctx, b.db, externalID, "mods export", false, err.Error())
			return
		}
		respondEphemeral(ctx, s, i, "SMM profile sent to your DMs.")
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
		respondEphemeral(ctx, s, i, "Could not attach SMM profile.")
		_ = LogBotCommand(ctx, b.db, externalID, "mods export", false, err.Error())
		return
	}
	_ = LogBotCommand(ctx, b.db, externalID, "mods export", true, filename)
}

func formatModsListEmbed(list mods.ListResponse) *notify.DiscordEmbed {
	const maxFieldLen = 1024

	var cached string
	if list.CachedAt != "" {
		if t, err := time.Parse(time.RFC3339, list.CachedAt); err == nil {
			cached = t.UTC().Format("Jan 2, 2006 · 15:04 UTC")
		}
	}
	title := "📦 Server mods"
	if cached != "" {
		title = fmt.Sprintf("📦 Server mods (%s)", cached)
	}

	desc := fmt.Sprintf(
		"Game build: **%s**\nSML: **%s**\n\nInstall ALL mods below at matching versions — or use `/mods action:export` for an SMM profile.",
		list.GameBuild, list.SMLVersion,
	)

	modLines := make([]string, 0, len(list.Mods))
	for _, m := range list.Mods {
		modLines = append(modLines, fmt.Sprintf("%s — %s", m.Name, m.Version))
	}

	fields := make([]notify.DiscordEmbedField, 0, 2)
	remaining := strings.Join(modLines, "\n")
	const maxModFields = 5
	truncated := false
	for len(remaining) > 0 {
		if len(fields) >= maxModFields {
			truncated = true
			break
		}
		chunk := remaining
		if len(chunk) > maxFieldLen {
			chunk = chunk[:maxFieldLen]
			if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
				chunk = chunk[:idx]
			}
		}
		name := "Mods"
		if len(fields) > 0 {
			name = "Mods (continued)"
		}
		fields = append(fields, notify.DiscordEmbedField{Name: name, Value: chunk})
		remaining = strings.TrimPrefix(remaining, chunk)
		remaining = strings.TrimPrefix(remaining, "\n")
	}

	footer := "Full list on the dashboard"
	if url := PublicURL(); url != "" {
		footer = fmt.Sprintf("Full list: %s/mods", url)
	}
	if truncated {
		desc += "\n\n⚠️ List truncated — open the dashboard for all mods."
	}
	return &notify.DiscordEmbed{
		Title:       title,
		Description: desc,
		Fields:      fields,
		Footer:      footer,
	}
}

func respondEphemeralEmbed(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, embed *notify.DiscordEmbed) {
	if embed == nil {
		respondEphemeral(ctx, s, i, "No mod data available.")
		return
	}
	dgEmbed := &discordgo.MessageEmbed{
		Title:       embed.Title,
		Description: embed.Description,
		Footer:      &discordgo.MessageEmbedFooter{Text: embed.Footer},
	}
	for _, f := range embed.Fields {
		dgEmbed.Fields = append(dgEmbed.Fields, &discordgo.MessageEmbedField{
			Name:   f.Name,
			Value:  f.Value,
			Inline: f.Inline,
		})
	}
	if interactionDeferred(ctx) {
		embeds := []*discordgo.MessageEmbed{dgEmbed}
		_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &embeds,
		})
		if err != nil {
			log.Printf("discord bot: ephemeral embed edit: %v", err)
		}
		return
	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{dgEmbed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not display mod list.")
	}
}
