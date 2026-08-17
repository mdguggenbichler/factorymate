package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const discordProviderType = "discord"

// DiscordConfig is the inner config object stored in notification_targets.config_json (§5.1).
type DiscordConfig struct {
	WebhookURL        string `json:"webhook_url"`
	UsernameOverride  string `json:"username_override,omitempty"`
	AvatarURLOverride string `json:"avatar_url_override,omitempty"`
}

// DiscordProvider posts rendered messages to Discord incoming webhooks (§5.1).
type DiscordProvider struct {
	httpClient *http.Client
}

// NewDiscordProvider constructs a Discord webhook provider.
func NewDiscordProvider() *DiscordProvider {
	return &DiscordProvider{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Type returns the provider identifier.
func (p *DiscordProvider) Type() string {
	return discordProviderType
}

type discordWebhookPayload struct {
	Content   string            `json:"content,omitempty"`
	Embeds    []discordAPIEmbed `json:"embeds,omitempty"`
	Username  string            `json:"username,omitempty"`
	AvatarURL string            `json:"avatar_url,omitempty"`
}

type discordAPIEmbed struct {
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Color       int               `json:"color,omitempty"`
	Fields      []discordAPIField `json:"fields,omitempty"`
}

type discordAPIField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// Send posts msg to the target's Discord webhook URL.
func (p *DiscordProvider) Send(ctx context.Context, target NotificationTarget, msg RenderedMessage) error {
	if target.ProviderType != discordProviderType {
		return fmt.Errorf("discord provider cannot send to target type %q", target.ProviderType)
	}

	var cfg DiscordConfig
	if err := json.Unmarshal([]byte(target.ConfigJSON), &cfg); err != nil {
		return fmt.Errorf("parse discord config: %w", err)
	}
	if strings.TrimSpace(cfg.WebhookURL) == "" {
		return fmt.Errorf("discord webhook_url is required")
	}

	payload, err := buildDiscordPayload(msg)
	if err != nil {
		return err
	}
	if cfg.UsernameOverride != "" {
		payload.Username = cfg.UsernameOverride
	}
	if cfg.AvatarURLOverride != "" {
		payload.AvatarURL = cfg.AvatarURLOverride
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("discord webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("discord webhook HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func buildDiscordPayload(msg RenderedMessage) (discordWebhookPayload, error) {
	var payload discordWebhookPayload

	if msg.Embed != nil {
		embed, err := toDiscordAPIEmbed(*msg.Embed)
		if err != nil {
			return payload, err
		}
		payload.Embeds = []discordAPIEmbed{embed}
	}

	if msg.Plain != "" {
		payload.Content = msg.Plain
	}

	if payload.Content == "" && len(payload.Embeds) == 0 {
		return payload, fmt.Errorf("rendered message has no plain text or embed content")
	}

	return payload, nil
}

func toDiscordAPIEmbed(embed DiscordEmbed) (discordAPIEmbed, error) {
	color, err := hexColorToDiscordInt(embed.Color)
	if err != nil {
		return discordAPIEmbed{}, err
	}

	apiEmbed := discordAPIEmbed{
		Title:       truncate(embed.Title, discordTitleMaxLen),
		Description: truncate(embed.Description, discordDescriptionMaxLen),
		Color:       color,
	}

	for _, field := range embed.Fields {
		if strings.TrimSpace(field.Value) == "" {
			continue
		}
		if len(apiEmbed.Fields) >= discordMaxFields {
			break
		}
		apiEmbed.Fields = append(apiEmbed.Fields, discordAPIField{
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

// SampleRenderedMessage returns a hardcoded player_joined embed for manual and automated tests.
func SampleRenderedMessage() RenderedMessage {
	return RenderedMessage{
		Embed: &DiscordEmbed{
			Title:       "🟢 NEW PLAYER DETECTED",
			Description: "**Guggi** has entered the factory.",
			Color:       "#57F287",
			Fields: []DiscordEmbedField{
				{Name: "Players online", Value: "3", Inline: true},
			},
		},
	}
}
