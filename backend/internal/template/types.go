package template

// Template is the stored shape for plain + embed variants (spec §5.4).
type Template struct {
	Plain string         `json:"plain,omitempty"`
	Embed *EmbedTemplate `json:"embed,omitempty"`
}

// EmbedTemplate is the structured embed model before variable substitution.
type EmbedTemplate struct {
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	Color         string       `json:"color"`
	Fields        []EmbedField `json:"fields"`
	Footer        string       `json:"footer,omitempty"`
	ShowTimestamp bool         `json:"show_timestamp,omitempty"`
}

// EmbedField is a single embed field in the template model.
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// RenderedMessage is the output of template rendering (spec §2.3, §5.4).
type RenderedMessage struct {
	Plain string
	Embed *DiscordEmbed
}

// DiscordEmbed is a rendered Discord embed payload.
type DiscordEmbed struct {
	Title       string
	Description string
	Color       string
	Fields      []DiscordEmbedField
	Footer      string
	Timestamp   string // ISO 8601 for Discord API when show_timestamp is set
}

// DiscordEmbedField is a rendered embed field.
type DiscordEmbedField struct {
	Name   string
	Value  string
	Inline bool
}
