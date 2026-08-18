package connection

import (
	"strings"
	"testing"

	"factorymate/internal/notify"
)

func TestEmbedHasPasswordCapacityWith24Fields(t *testing.T) {
	fields := make([]notify.DiscordEmbedField, 24)
	for i := range fields {
		fields[i] = notify.DiscordEmbedField{
			Name:  "Field",
			Value: "x",
		}
	}
	embed := &notify.DiscordEmbed{
		Title:  "Connection details",
		Fields: fields,
	}
	if !embedHasPasswordCapacity(embed, "short") {
		t.Fatal("expected password field to fit with 24 existing fields")
	}
}

func TestEmbedHasPasswordCapacityRejectsAtFieldLimit(t *testing.T) {
	fields := make([]notify.DiscordEmbedField, discordEmbedMaxFields)
	for i := range fields {
		fields[i] = notify.DiscordEmbedField{Name: "Field", Value: "x"}
	}
	embed := &notify.DiscordEmbed{Fields: fields}
	if embedHasPasswordCapacity(embed, "secret") {
		t.Fatal("expected no capacity when field count is already at limit")
	}
}

func TestEnrichChangeDMIncludesPassword(t *testing.T) {
	old := Details{GamePassword: "old-secret"}
	new := Details{
		GameHost:     "play.example.com",
		GamePort:     7777,
		GamePassword: "new-secret",
	}

	msg := enrichChangeDM(
		notify.RenderedMessage{
			Embed: &notify.DiscordEmbed{
				Title: "Connection details updated",
			},
		},
		old,
		new,
	)

	if msg.Embed == nil {
		t.Fatal("expected embed message")
	}
	foundPasswordField := false
	for _, f := range msg.Embed.Fields {
		if f.Name == "Password" && strings.Contains(f.Value, "new-secret") {
			foundPasswordField = true
		}
	}
	if !foundPasswordField {
		t.Fatalf("embed fields missing password: %+v", msg.Embed.Fields)
	}
	if !strings.Contains(msg.Embed.Description, "Password was changed") {
		t.Fatalf("expected password-changed note in description: %q", msg.Embed.Description)
	}
}
