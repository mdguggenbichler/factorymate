package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"factorymate/internal/template"
)

// ErrNoTargets is returned when a test send has no assigned enabled targets.
var ErrNoTargets = errors.New("no targets assigned")

// Dispatcher routes detected poller events to notification providers (spec §5.3).
type Dispatcher struct {
	DB        *sql.DB
	Providers map[string]Provider
	Now       func() time.Time
}

// NewDispatcher constructs a Dispatcher with the given provider registry.
func NewDispatcher(db *sql.DB, providers map[string]Provider) *Dispatcher {
	return &Dispatcher{
		DB:        db,
		Providers: providers,
		Now:       time.Now,
	}
}

// HandleEvent implements poller.EventHandler — render and send for enabled types/targets.
func (d *Dispatcher) HandleEvent(ctx context.Context, messageTypeKey string, vars map[string]string) error {
	enabled, defaultJSON, err := d.loadMessageType(ctx, messageTypeKey)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	targets, err := d.loadTargets(ctx, messageTypeKey)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}

	tmpl, err := d.loadEffectiveTemplate(ctx, messageTypeKey, defaultJSON)
	if err != nil {
		return err
	}

	rendered := template.Render(tmpl, d.mergeSystemVariables(ctx, vars))
	for _, target := range targets {
		_ = d.dispatchToTarget(ctx, messageTypeKey, target, rendered)
	}
	return nil
}

// SendRenderedTest sends a pre-rendered message to all assigned enabled targets.
// Unlike HandleEvent, it does not check message_types.enabled.
func (d *Dispatcher) SendRenderedTest(ctx context.Context, messageTypeKey string, rendered template.RenderedMessage) error {
	targets, err := d.loadTargets(ctx, messageTypeKey)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return ErrNoTargets
	}

	var firstErr error
	for _, target := range targets {
		if err := d.dispatchToTarget(ctx, messageTypeKey, target, rendered); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (d *Dispatcher) loadMessageType(ctx context.Context, key string) (bool, string, error) {
	var enabled bool
	var defaultJSON string
	err := d.DB.QueryRowContext(ctx, `
		SELECT enabled, default_template_json FROM message_types WHERE key = ?`, key,
	).Scan(&enabled, &defaultJSON)
	if err == sql.ErrNoRows {
		return false, "", fmt.Errorf("unknown message type %q", key)
	}
	if err != nil {
		return false, "", fmt.Errorf("load message type %q: %w", key, err)
	}
	return enabled, defaultJSON, nil
}

func (d *Dispatcher) loadTargets(ctx context.Context, messageTypeKey string) ([]NotificationTarget, error) {
	rows, err := d.DB.QueryContext(ctx, `
		SELECT nt.id, nt.name, nt.provider_type, nt.config_json, nt.enabled
		FROM message_type_targets mtt
		JOIN notification_targets nt ON nt.id = mtt.target_id
		WHERE mtt.message_type_key = ? AND nt.enabled = 1`,
		messageTypeKey,
	)
	if err != nil {
		return nil, fmt.Errorf("load targets for %q: %w", messageTypeKey, err)
	}
	defer rows.Close()

	var targets []NotificationTarget
	for rows.Next() {
		var t NotificationTarget
		if err := rows.Scan(&t.ID, &t.Name, &t.ProviderType, &t.ConfigJSON, &t.Enabled); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func (d *Dispatcher) loadEffectiveTemplate(ctx context.Context, key, defaultJSON string) (template.Template, error) {
	var defaults template.Template
	if err := json.Unmarshal([]byte(defaultJSON), &defaults); err != nil {
		return template.Template{}, fmt.Errorf("parse default template for %q: %w", key, err)
	}

	var overrideJSON sql.NullString
	err := d.DB.QueryRowContext(ctx,
		`SELECT template_json FROM message_templates WHERE message_type_key = ?`, key,
	).Scan(&overrideJSON)
	if err != nil && err != sql.ErrNoRows {
		return template.Template{}, fmt.Errorf("load template override for %q: %w", key, err)
	}

	if overrideJSON.Valid {
		return template.Merge(defaults, []byte(overrideJSON.String))
	}
	return defaults, nil
}

func (d *Dispatcher) dispatchToTarget(ctx context.Context, messageTypeKey string, target NotificationTarget, rendered template.RenderedMessage) error {
	msg := providerMessage(target.ProviderType, rendered)
	preview := renderedPreview(target.ProviderType, rendered)

	provider, ok := d.Providers[target.ProviderType]
	if !ok {
		err := fmt.Errorf("unknown provider type %q", target.ProviderType)
		d.recordLog(ctx, messageTypeKey, target.ID, preview, false, err)
		return err
	}

	sendErr := provider.Send(ctx, target, msg)
	d.recordLog(ctx, messageTypeKey, target.ID, preview, sendErr == nil, sendErr)
	return sendErr
}

func providerMessage(providerType string, rendered template.RenderedMessage) RenderedMessage {
	if providerType == "discord" && rendered.Embed != nil {
		return RenderedMessage{Embed: toNotifyEmbed(rendered.Embed)}
	}
	return RenderedMessage{Plain: rendered.Plain}
}

func toNotifyEmbed(embed *template.DiscordEmbed) *DiscordEmbed {
	if embed == nil {
		return nil
	}
	out := &DiscordEmbed{
		Title:       embed.Title,
		Description: embed.Description,
		Color:       embed.Color,
		Footer:      embed.Footer,
		Timestamp:   embed.Timestamp,
	}
	for _, f := range embed.Fields {
		out.Fields = append(out.Fields, DiscordEmbedField{
			Name:   f.Name,
			Value:  f.Value,
			Inline: f.Inline,
		})
	}
	return out
}

func (d *Dispatcher) mergeSystemVariables(ctx context.Context, vars map[string]string) map[string]string {
	now := d.Now().UTC()
	out := make(map[string]string, len(vars)+3)
	for k, v := range vars {
		out[k] = v
	}
	out["Timestamp"] = formatDispatchTimestamp(now)
	out["TimestampISO"] = now.Format(time.RFC3339)
	if out["ServerName"] == "" {
		if name := d.loadServerName(ctx); name != "" {
			out["ServerName"] = name
		}
	}
	return out
}

func (d *Dispatcher) loadServerName(ctx context.Context) string {
	var name string
	err := d.DB.QueryRowContext(ctx, `SELECT server_name FROM app_settings WHERE id = 1`).Scan(&name)
	if err != nil {
		return ""
	}
	return name
}

func formatDispatchTimestamp(t time.Time) string {
	return t.Format("Jan 2, 2006 · 15:04 UTC")
}

func renderedPreview(providerType string, rendered template.RenderedMessage) string {
	if providerType == "discord" && rendered.Embed != nil {
		parts := make([]string, 0, 2)
		if rendered.Embed.Title != "" {
			parts = append(parts, rendered.Embed.Title)
		}
		if rendered.Embed.Description != "" {
			parts = append(parts, rendered.Embed.Description)
		}
		return strings.Join(parts, " — ")
	}
	return rendered.Plain
}

func (d *Dispatcher) recordLog(ctx context.Context, messageTypeKey string, targetID int64, preview string, success bool, sendErr error) {
	var errText sql.NullString
	if sendErr != nil {
		errText = sql.NullString{String: sendErr.Error(), Valid: true}
	}

	sentAt := d.Now().UTC().Format(time.RFC3339)
	_, err := d.DB.ExecContext(ctx, `
		INSERT INTO notification_log (message_type_key, target_id, rendered_preview, success, error, sent_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		messageTypeKey, targetID, preview, success, errText, sentAt,
	)
	if err != nil {
		// Best-effort audit log; do not fail the poll loop.
		_ = err
	}
}
