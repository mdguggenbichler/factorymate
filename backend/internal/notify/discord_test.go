package notify

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDiscordProvider_SendSampleEmbed(t *testing.T) {
	t.Parallel()

	mock := NewMockDiscordSession()
	cfgJSON, err := json.Marshal(DiscordConfig{ChannelID: "123456789"})
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

	provider := NewDiscordProvider(mock)
	if err := provider.Send(context.Background(), target, SampleRenderedMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(mock.ChannelCalls) != 1 {
		t.Fatalf("channel calls = %d, want 1", len(mock.ChannelCalls))
	}
	if mock.ChannelCalls[0].ChannelID != "123456789" {
		t.Errorf("channel id = %q, want 123456789", mock.ChannelCalls[0].ChannelID)
	}

	embeds := mock.ChannelCalls[0].Message.Embeds
	if len(embeds) != 1 {
		t.Fatalf("embeds len = %d, want 1", len(embeds))
	}
	embed := embeds[0]
	if embed.Title != "👤 A player joined the server" {
		t.Errorf("title = %q", embed.Title)
	}
	if embed.Footer == nil || embed.Footer.Text == "" {
		t.Error("expected footer text")
	}
	if embed.Timestamp == "" {
		t.Error("expected timestamp")
	}
	if embed.Color != 0x57F287 {
		t.Errorf("color = %d, want %d", embed.Color, 0x57F287)
	}
	if len(embed.Fields) != 3 {
		t.Fatalf("fields len = %d, want 3", len(embed.Fields))
	}
}

func TestDiscordProvider_SendDirect(t *testing.T) {
	t.Parallel()

	mock := NewMockDiscordSession()
	provider := NewDiscordProvider(mock)
	msg := RenderedMessage{Plain: "hello from DM"}

	if err := provider.SendDirect(context.Background(), "discord", "user-999", msg); err != nil {
		t.Fatalf("SendDirect: %v", err)
	}
	if len(mock.DMUserIDs) != 1 || mock.DMUserIDs[0] != "user-999" {
		t.Fatalf("dm user ids = %v", mock.DMUserIDs)
	}
	if len(mock.ChannelCalls) != 1 || mock.ChannelCalls[0].ChannelID != mock.DMChannelID {
		t.Fatalf("channel calls = %+v", mock.ChannelCalls)
	}
	if mock.ChannelCalls[0].Message.Content != "hello from DM" {
		t.Errorf("content = %q", mock.ChannelCalls[0].Message.Content)
	}
}

func TestDiscordProvider_OmitsEmptyFieldValues(t *testing.T) {
	t.Parallel()

	mock := NewMockDiscordSession()
	cfgJSON, _ := json.Marshal(DiscordConfig{ChannelID: "123"})
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

	if err := NewDiscordProvider(mock).Send(context.Background(), target, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	fields := mock.ChannelCalls[0].Message.Embeds[0].Fields
	if len(fields) != 1 {
		t.Fatalf("fields len = %d, want 1 (empty value omitted)", len(fields))
	}
}

func TestDiscordProvider_SendPlainContent(t *testing.T) {
	t.Parallel()

	mock := NewMockDiscordSession()
	cfgJSON, _ := json.Marshal(DiscordConfig{ChannelID: "123"})
	target := NotificationTarget{
		ProviderType: "discord",
		ConfigJSON:   string(cfgJSON),
	}

	msg := RenderedMessage{Plain: "🟢 **Guggi** has entered the factory. (3 online)"}
	if err := NewDiscordProvider(mock).Send(context.Background(), target, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if mock.ChannelCalls[0].Message.Content != msg.Plain {
		t.Errorf("content = %q, want %q", mock.ChannelCalls[0].Message.Content, msg.Plain)
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
	if NewDiscordProvider(nil).Type() != "discord" {
		t.Fatalf("Type() = %q, want discord", NewDiscordProvider(nil).Type())
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

func TestRedactForLog(t *testing.T) {
	t.Parallel()

	in := `{"game_host":"x","game_password":"hunter2","game_port":7777}`
	out := RedactForLog(in)
	if strings.Contains(out, "hunter2") {
		t.Fatalf("password leaked in %q", out)
	}
	if !strings.Contains(out, `[REDACTED]`) {
		t.Fatalf("expected redaction marker in %q", out)
	}
}

// Ensure MockDiscordSession satisfies DiscordSession.
var _ DiscordSession = (*MockDiscordSession)(nil)
var _ DirectMessageProvider = (*DiscordProvider)(nil)
