package connection

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"factorymate/internal/notify"
	"factorymate/internal/template"
)

func (s *Service) renderChangeMessage(ctx context.Context, old, new Details) notify.RenderedMessage {
	tmpl, err := s.loadEffectiveTemplate(ctx)
	if err != nil {
		return FormatChangeDM(new, old)
	}
	rendered := template.Render(tmpl, s.buildTemplateVars(ctx, new))
	msg := toNotifyMessage(rendered)
	return enrichChangeDM(msg, old, new)
}

func (s *Service) loadEffectiveTemplate(ctx context.Context) (template.Template, error) {
	var defaultJSON string
	err := s.DB.QueryRowContext(ctx, `
		SELECT default_template_json FROM message_types WHERE key = ?`, messageTypeKey,
	).Scan(&defaultJSON)
	if err == sql.ErrNoRows {
		return template.Template{}, fmt.Errorf("message type %q not found", messageTypeKey)
	}
	if err != nil {
		return template.Template{}, fmt.Errorf("load message type %q: %w", messageTypeKey, err)
	}

	var defaults template.Template
	if err := json.Unmarshal([]byte(defaultJSON), &defaults); err != nil {
		return template.Template{}, fmt.Errorf("parse default template: %w", err)
	}

	var overrideJSON sql.NullString
	err = s.DB.QueryRowContext(ctx,
		`SELECT template_json FROM message_templates WHERE message_type_key = ?`, messageTypeKey,
	).Scan(&overrideJSON)
	if err != nil && err != sql.ErrNoRows {
		return template.Template{}, fmt.Errorf("load template override: %w", err)
	}
	if overrideJSON.Valid {
		return template.Merge(defaults, []byte(overrideJSON.String))
	}
	return defaults, nil
}

func (s *Service) buildTemplateVars(ctx context.Context, details Details) map[string]string {
	now := s.Now().UTC()
	vars := map[string]string{
		"GameHost":     details.GameHost,
		"GamePort":     fmt.Sprintf("%d", details.GamePort),
		"Notes":        strings.TrimSpace(details.Notes),
		"Timestamp":    formatTemplateTimestamp(now),
		"TimestampISO": now.Format(time.RFC3339),
	}
	if name := s.loadServerName(ctx); name != "" {
		vars["ServerName"] = name
	}
	return vars
}

func (s *Service) loadServerName(ctx context.Context) string {
	var name string
	err := s.DB.QueryRowContext(ctx, `SELECT server_name FROM app_settings WHERE id = 1`).Scan(&name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

func formatTemplateTimestamp(t time.Time) string {
	return t.UTC().Format("Jan 2, 2006 · 15:04 UTC")
}

func toNotifyMessage(rendered template.RenderedMessage) notify.RenderedMessage {
	if rendered.Embed != nil {
		return notify.RenderedMessage{Embed: toNotifyEmbed(rendered.Embed)}
	}
	return notify.RenderedMessage{Plain: rendered.Plain}
}

func toNotifyEmbed(embed *template.DiscordEmbed) *notify.DiscordEmbed {
	if embed == nil {
		return nil
	}
	out := &notify.DiscordEmbed{
		Title:       embed.Title,
		Description: embed.Description,
		Color:       embed.Color,
		Footer:      embed.Footer,
		Timestamp:   embed.Timestamp,
	}
	for _, f := range embed.Fields {
		out.Fields = append(out.Fields, notify.DiscordEmbedField{
			Name:   f.Name,
			Value:  f.Value,
			Inline: f.Inline,
		})
	}
	return out
}

// enrichChangeDM appends user-facing join details omitted from templates (§8.3, Appendix A).
// Password is never in template vars — only in authorized DM bodies, not log previews.
func enrichChangeDM(msg notify.RenderedMessage, old, new Details) notify.RenderedMessage {
	if strings.TrimSpace(new.GamePassword) == "" {
		return msg
	}

	passwordLine := fmt.Sprintf("Password: ||%s||", new.GamePassword)
	var prefix string
	if old.GamePassword != new.GamePassword && strings.TrimSpace(old.GamePassword) != "" {
		prefix = "Password was changed.\n"
	}

	if strings.TrimSpace(msg.Plain) != "" {
		msg.Plain = strings.TrimSpace(msg.Plain) + "\n\n" + prefix + passwordLine
		return msg
	}

	if msg.Embed != nil {
		if prefix != "" {
			if msg.Embed.Description != "" {
				msg.Embed.Description += "\n\n" + prefix
			} else {
				msg.Embed.Description = prefix
			}
		}
		msg.Embed.Fields = append(msg.Embed.Fields, notify.DiscordEmbedField{
			Name:  "Password",
			Value: "||" + new.GamePassword + "||",
		})
		return msg
	}

	msg.Plain = prefix + passwordLine
	return msg
}
