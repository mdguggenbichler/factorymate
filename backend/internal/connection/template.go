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
	tmpl, err := s.loadEffectiveTemplate(ctx, messageTypeKey)
	if err != nil {
		return FormatChangeDM(new, old)
	}
	rendered := template.Render(tmpl, s.buildTemplateVars(ctx, new))
	msg := toNotifyMessage(rendered)
	return enrichChangeDM(msg, old, new)
}

// RenderDetailsMessage builds the user-facing join-details DM from the connection_details template.
func (s *Service) RenderDetailsMessage(ctx context.Context, details Details) notify.RenderedMessage {
	return s.renderDetailsMessage(ctx, details)
}

func (s *Service) renderDetailsMessage(ctx context.Context, details Details) notify.RenderedMessage {
	tmpl, err := s.loadEffectiveTemplate(ctx, messageTypeKeyDetails)
	if err != nil {
		return FormatDetailsDM(details)
	}
	rendered := template.Render(tmpl, s.buildTemplateVars(ctx, details))
	msg := toNotifyMessage(rendered)
	return enrichDetailsDM(msg, details)
}

func (s *Service) loadEffectiveTemplate(ctx context.Context, typeKey string) (template.Template, error) {
	var defaultJSON string
	err := s.DB.QueryRowContext(ctx, `
		SELECT default_template_json FROM message_types WHERE key = ?`, typeKey,
	).Scan(&defaultJSON)
	if err == sql.ErrNoRows {
		return template.Template{}, fmt.Errorf("message type %q not found", typeKey)
	}
	if err != nil {
		return template.Template{}, fmt.Errorf("load message type %q: %w", typeKey, err)
	}

	var defaults template.Template
	if err := json.Unmarshal([]byte(defaultJSON), &defaults); err != nil {
		return template.Template{}, fmt.Errorf("parse default template: %w", err)
	}

	var overrideJSON sql.NullString
	err = s.DB.QueryRowContext(ctx,
		`SELECT template_json FROM message_templates WHERE message_type_key = ?`, typeKey,
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
		if embedHasPasswordCapacity(msg.Embed, new.GamePassword) {
			msg.Embed.Fields = append(msg.Embed.Fields, notify.DiscordEmbedField{
				Name:  "Password",
				Value: "||" + new.GamePassword + "||",
			})
			return msg
		}
		msg.Plain = prefix + passwordLine
		msg.Embed = nil
		return msg
	}

	msg.Plain = prefix + passwordLine
	return msg
}

// enrichDetailsDM appends password to join-details messages (never in template vars).
func enrichDetailsDM(msg notify.RenderedMessage, details Details) notify.RenderedMessage {
	if strings.TrimSpace(details.GamePassword) == "" {
		return msg
	}

	passwordLine := fmt.Sprintf("Password: ||%s||", details.GamePassword)

	if strings.TrimSpace(msg.Plain) != "" {
		msg.Plain = strings.TrimSpace(msg.Plain) + "\n\n" + passwordLine
		return msg
	}

	if msg.Embed != nil {
		if embedHasPasswordCapacity(msg.Embed, details.GamePassword) {
			msg.Embed.Fields = append(msg.Embed.Fields, notify.DiscordEmbedField{
				Name:  "Password",
				Value: "||" + details.GamePassword + "||",
			})
			return msg
		}
		msg.Plain = passwordLine
		msg.Embed = nil
		return msg
	}

	msg.Plain = passwordLine
	return msg
}

const (
	discordEmbedMaxFields     = 25
	discordEmbedMaxTotalChars = 6000
)

func embedHasPasswordCapacity(embed *notify.DiscordEmbed, password string) bool {
	if embed == nil {
		return true
	}
	if len(embed.Fields) >= discordEmbedMaxFields {
		return false
	}
	passwordFieldSize := len("Password") + len("||") + len(password) + len("||")
	total := len(embed.Title) + len(embed.Description) + len(embed.Footer) + passwordFieldSize
	for _, f := range embed.Fields {
		total += len(f.Name) + len(f.Value)
	}
	return total <= discordEmbedMaxTotalChars
}
