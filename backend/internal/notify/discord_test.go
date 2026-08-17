package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscordProvider_SendSampleEmbed(t *testing.T) {
	t.Parallel()

	var got discordWebhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfgJSON, err := json.Marshal(DiscordConfig{
		WebhookURL:       srv.URL,
		UsernameOverride: "F.I.C.S.I.T. Oracle",
	})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	target := NotificationTarget{
		ID:           1,
		Name:         "Test Channel",
		ProviderType: "discord",
		ConfigJSON:   string(cfgJSON),
		Enabled:      true,
	}

	provider := NewDiscordProvider()
	if err := provider.Send(context.Background(), target, SampleRenderedMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if got.Username != "F.I.C.S.I.T. Oracle" {
		t.Errorf("username = %q, want F.I.C.S.I.T. Oracle", got.Username)
	}
	if len(got.Embeds) != 1 {
		t.Fatalf("embeds len = %d, want 1", len(got.Embeds))
	}

	embed := got.Embeds[0]
	if embed.Title != "🟢 NEW PLAYER DETECTED" {
		t.Errorf("title = %q", embed.Title)
	}
	if embed.Description != "**Guggi** has entered the factory." {
		t.Errorf("description = %q", embed.Description)
	}
	if embed.Color != 0x57F287 {
		t.Errorf("color = %d, want %d", embed.Color, 0x57F287)
	}
	if len(embed.Fields) != 1 {
		t.Fatalf("fields len = %d, want 1", len(embed.Fields))
	}
	if embed.Fields[0].Name != "Players online" || embed.Fields[0].Value != "3" || !embed.Fields[0].Inline {
		t.Errorf("field = %+v", embed.Fields[0])
	}
}

func TestDiscordProvider_OmitsEmptyFieldValues(t *testing.T) {
	t.Parallel()

	var got discordWebhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(DiscordConfig{WebhookURL: srv.URL})
	target := NotificationTarget{
		ProviderType: "discord",
		ConfigJSON:   string(cfgJSON),
	}

	msg := RenderedMessage{
		Embed: &DiscordEmbed{
			Title:       "Test",
			Description: "Body",
			Color:       "#5865F2",
			Fields: []DiscordEmbedField{
				{Name: "Shown", Value: "yes", Inline: true},
				{Name: "Hidden", Value: "   ", Inline: false},
			},
		},
	}

	if err := NewDiscordProvider().Send(context.Background(), target, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(got.Embeds[0].Fields) != 1 {
		t.Fatalf("fields len = %d, want 1 (empty value omitted)", len(got.Embeds[0].Fields))
	}
}

func TestDiscordProvider_SendPlainContent(t *testing.T) {
	t.Parallel()

	var got discordWebhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(DiscordConfig{WebhookURL: srv.URL})
	target := NotificationTarget{
		ProviderType: "discord",
		ConfigJSON:   string(cfgJSON),
	}

	msg := RenderedMessage{Plain: "🟢 **Guggi** has entered the factory. (3 online)"}
	if err := NewDiscordProvider().Send(context.Background(), target, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Content != msg.Plain {
		t.Errorf("content = %q, want %q", got.Content, msg.Plain)
	}
}

func TestDiscordProvider_EnforcesLimits(t *testing.T) {
	t.Parallel()

	longTitle := strings.Repeat("x", discordTitleMaxLen+100)
	longDescription := strings.Repeat("x", discordDescriptionMaxLen+100)
	longFieldName := strings.Repeat("x", discordFieldNameMaxLen+100)
	longFieldValue := strings.Repeat("x", discordFieldValueMaxLen+100)

	apiEmbed, err := toDiscordAPIEmbed(DiscordEmbed{
		Title:       longTitle,
		Description: longDescription,
		Color:       "#57F287",
		Fields: []DiscordEmbedField{
			{Name: longFieldName, Value: longFieldValue, Inline: true},
		},
	})
	if err != nil {
		t.Fatalf("toDiscordAPIEmbed: %v", err)
	}

	if len(apiEmbed.Title) != discordTitleMaxLen {
		t.Errorf("title len = %d, want %d", len(apiEmbed.Title), discordTitleMaxLen)
	}
	if len(apiEmbed.Description) != discordDescriptionMaxLen {
		t.Errorf("description len = %d, want %d", len(apiEmbed.Description), discordDescriptionMaxLen)
	}
	if len(apiEmbed.Fields[0].Name) != discordFieldNameMaxLen {
		t.Errorf("field name len = %d, want %d", len(apiEmbed.Fields[0].Name), discordFieldNameMaxLen)
	}
	if len(apiEmbed.Fields[0].Value) != discordFieldValueMaxLen {
		t.Errorf("field value len = %d, want %d", len(apiEmbed.Fields[0].Value), discordFieldValueMaxLen)
	}
}

func TestDiscordProvider_Type(t *testing.T) {
	t.Parallel()
	if NewDiscordProvider().Type() != "discord" {
		t.Fatalf("Type() = %q, want discord", NewDiscordProvider().Type())
	}
}

func TestHexColorToDiscordInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want int
	}{
		{"#57F287", 0x57F287},
		{"57F287", 0x57F287},
		{"", 0},
	}
	for _, tc := range tests {
		got, err := hexColorToDiscordInt(tc.in)
		if err != nil {
			t.Fatalf("hexColorToDiscordInt(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("hexColorToDiscordInt(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
