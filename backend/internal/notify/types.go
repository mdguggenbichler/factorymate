package notify

import "context"

// NotificationTarget is a named destination with provider-specific config (spec §2.3, §5.1).
type NotificationTarget struct {
	ID           int64
	Name         string
	ProviderType string // "discord" in v1
	ConfigJSON   string // provider-specific JSON (see §5.1)
	Enabled      bool
}

// RenderedMessage is the output of template rendering for provider dispatch (spec §2.3).
type RenderedMessage struct {
	Plain string        // populated for plain-text providers
	Embed *DiscordEmbed // populated when provider_type == "discord"
}

// DiscordEmbed is a rendered Discord embed (spec §2.3).
type DiscordEmbed struct {
	Title       string
	Description string
	Color       string // hex, e.g. "#57F287"
	Fields      []DiscordEmbedField
}

// DiscordEmbedField is a single embed field.
type DiscordEmbedField struct {
	Name   string
	Value  string
	Inline bool
}

// Provider sends a rendered message to a notification target (spec §2.3).
type Provider interface {
	Type() string
	Send(ctx context.Context, target NotificationTarget, msg RenderedMessage) error
}
