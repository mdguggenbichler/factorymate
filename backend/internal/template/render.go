package template

// Render produces plain and embed variants from a resolved template (§5.4).
func Render(tmpl Template, vars map[string]string) RenderedMessage {
	msg := RenderedMessage{
		Plain: Substitute(tmpl.Plain, vars),
	}
	if tmpl.Embed != nil {
		msg.Embed = RenderEmbed(tmpl.Embed, vars)
	}
	return msg
}

// RenderEmbed renders a structured embed, omitting fields with empty values (§5.4).
func RenderEmbed(embed *EmbedTemplate, vars map[string]string) *DiscordEmbed {
	if embed == nil {
		return nil
	}

	out := &DiscordEmbed{
		Title:       Substitute(embed.Title, vars),
		Description: Substitute(embed.Description, vars),
		Color:       Substitute(embed.Color, vars),
	}

	for _, f := range embed.Fields {
		value := Substitute(f.Value, vars)
		if value == "" {
			continue
		}
		out.Fields = append(out.Fields, DiscordEmbedField{
			Name:   Substitute(f.Name, vars),
			Value:  value,
			Inline: f.Inline,
		})
	}
	return out
}
