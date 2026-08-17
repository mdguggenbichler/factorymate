package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const discordProviderType = "discord"

// DiscordConfig is the inner config object stored in notification_targets.config_json (§5.1).
type DiscordConfig struct {
	ChannelID string `json:"channel_id"`
	ThreadID  string `json:"thread_id,omitempty"`
}

// DiscordSession is the subset of discordgo.Session used for outbound messaging.
type DiscordSession interface {
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
	UserChannelCreate(recipientID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
}

// DiscordProvider posts rendered messages via a live discordgo session (§5.1).
type DiscordProvider struct {
	session DiscordSession
}

// SendEnabled, when set, gates outbound Discord sends on the bot kill switch.
var SendEnabled func(ctx context.Context) (bool, error)

// NewDiscordProvider constructs a Discord bot provider. session may be nil when the bot is offline.
func NewDiscordProvider(session DiscordSession) *DiscordProvider {
	return &DiscordProvider{session: session}
}

// Type returns the provider identifier.
func (p *DiscordProvider) Type() string {
	return discordProviderType
}

// Send posts msg to the target's Discord channel.
func (p *DiscordProvider) Send(ctx context.Context, target NotificationTarget, msg RenderedMessage) error {
	if err := p.checkSendEnabled(ctx); err != nil {
		return err
	}
	if target.ProviderType != discordProviderType {
		return fmt.Errorf("discord provider cannot send to target type %q", target.ProviderType)
	}
	if p.session == nil {
		return fmt.Errorf("discord bot is not connected")
	}

	var cfg DiscordConfig
	if err := json.Unmarshal([]byte(target.ConfigJSON), &cfg); err != nil {
		return fmt.Errorf("parse discord config: %w", err)
	}
	channelID := strings.TrimSpace(cfg.ChannelID)
	if channelID == "" {
		return fmt.Errorf("discord channel_id is required")
	}
	if cfg.ThreadID != "" {
		channelID = strings.TrimSpace(cfg.ThreadID)
	}

	send, err := buildDiscordMessageSend(msg)
	if err != nil {
		return err
	}

	_, err = p.session.ChannelMessageSendComplex(channelID, send, discordgo.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("discord channel send: %w", err)
	}
	return nil
}

// SendDirect delivers msg to an external user via DM.
func (p *DiscordProvider) SendDirect(ctx context.Context, platform, externalUserID string, msg RenderedMessage) error {
	if err := p.checkSendEnabled(ctx); err != nil {
		return err
	}
	if platform != "" && platform != discordProviderType {
		return fmt.Errorf("discord provider cannot DM platform %q", platform)
	}
	if strings.TrimSpace(externalUserID) == "" {
		return fmt.Errorf("external user id is required")
	}
	if p.session == nil {
		return fmt.Errorf("discord bot is not connected")
	}

	channel, err := p.session.UserChannelCreate(externalUserID, discordgo.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("discord create dm channel: %w", err)
	}

	send, err := buildDiscordMessageSend(msg)
	if err != nil {
		return err
	}

	_, err = p.session.ChannelMessageSendComplex(channel.ID, send, discordgo.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("discord dm send: %w", err)
	}
	return nil
}

func (p *DiscordProvider) checkSendEnabled(ctx context.Context) error {
	if SendEnabled == nil {
		return nil
	}
	enabled, err := SendEnabled(ctx)
	if err != nil {
		return fmt.Errorf("discord enabled check: %w", err)
	}
	if !enabled {
		return fmt.Errorf("discord bot is disabled")
	}
	return nil
}

func buildDiscordMessageSend(msg RenderedMessage) (*discordgo.MessageSend, error) {
	send := &discordgo.MessageSend{}

	if msg.Embed != nil {
		embed, err := toDiscordAPIEmbed(*msg.Embed)
		if err != nil {
			return nil, err
		}
		send.Embeds = []*discordgo.MessageEmbed{embed}
	}

	if msg.Plain != "" {
		send.Content = msg.Plain
	}

	if send.Content == "" && len(send.Embeds) == 0 {
		return nil, fmt.Errorf("rendered message has no plain text or embed content")
	}

	return send, nil
}

func toDiscordAPIEmbed(embed DiscordEmbed) (*discordgo.MessageEmbed, error) {
	color, err := hexColorToDiscordInt(embed.Color)
	if err != nil {
		return nil, err
	}

	apiEmbed := &discordgo.MessageEmbed{
		Title:       truncate(embed.Title, discordTitleMaxLen),
		Description: truncate(embed.Description, discordDescriptionMaxLen),
		Color:       color,
	}

	if footer := strings.TrimSpace(embed.Footer); footer != "" {
		apiEmbed.Footer = &discordgo.MessageEmbedFooter{
			Text: truncate(footer, discordFooterMaxLen),
		}
	}
	if embed.Timestamp != "" {
		apiEmbed.Timestamp = embed.Timestamp
	}

	for _, field := range embed.Fields {
		if strings.TrimSpace(field.Value) == "" {
			continue
		}
		if len(apiEmbed.Fields) >= discordMaxFields {
			break
		}
		apiEmbed.Fields = append(apiEmbed.Fields, &discordgo.MessageEmbedField{
			Name:   truncate(field.Name, discordFieldNameMaxLen),
			Value:  truncate(field.Value, discordFieldValueMaxLen),
			Inline: field.Inline,
		})
	}

	return apiEmbed, nil
}

func hexColorToDiscordInt(color string) (int, error) {
	hex := strings.TrimPrefix(strings.TrimSpace(color), "#")
	if hex == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(hex, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid embed color %q: %w", color, err)
	}
	return int(value), nil
}

var gamePasswordJSONField = regexp.MustCompile(`"game_password"\s*:\s*"[^"]*"`)

// RedactForLog removes sensitive values (e.g. game_password) from log-oriented strings (§8.6).
func RedactForLog(s string) string {
	return gamePasswordJSONField.ReplaceAllString(s, `"game_password":"[REDACTED]"`)
}

// SampleRenderedMessage returns a hardcoded player_joined embed for manual and automated tests.
func SampleRenderedMessage() RenderedMessage {
	return RenderedMessage{
		Embed: &DiscordEmbed{
			Title:     "👤 A player joined the server",
			Color:     "#57F287",
			Footer:    "🏭 CBC | Conveyor Belt Cult · Aug 17, 2026 · 14:37 UTC",
			Timestamp: "2026-08-17T14:37:00Z",
			Fields: []DiscordEmbedField{
				{Name: "👤 Player", Value: "Michael", Inline: true},
				{Name: "🏭 Factory", Value: "CBC | Conveyor Belt Cult", Inline: true},
				{Name: "👥 Online", Value: "4 players online", Inline: true},
			},
		},
	}
}
