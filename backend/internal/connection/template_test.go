package connection

import (
	"strings"
	"testing"

	"factorymate/internal/notify"
)

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
