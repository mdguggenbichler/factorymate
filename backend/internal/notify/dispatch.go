package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"factorymate/internal/notifications"
	"factorymate/internal/template"
)

// ErrNoTargets is returned when a test send has no assigned enabled targets.
var ErrNoTargets = errors.New("no targets assigned")

// Dispatcher routes detected poller events to notification providers (spec §5.3).
type Dispatcher struct {
	DB        *sql.DB
	Providers map[string]Provider
	Prefs     *notifications.Service
	Now       func() time.Time
}

// NewDispatcher constructs a Dispatcher with the given provider registry.
func NewDispatcher(db *sql.DB, providers map[string]Provider) *Dispatcher {
	return &Dispatcher{
		DB:        db,
		Providers: providers,
		Prefs:     notifications.NewService(db),
		Now:       time.Now,
	}
}

// HandleEvent implements poller.EventHandler — render and send for enabled types/targets.
func (d *Dispatcher) HandleEvent(ctx context.Context, messageTypeKey string, vars map[string]string) error {
	enabled, category, defaultJSON, err := d.loadMessageType(ctx, messageTypeKey)
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

	tmpl, err := d.loadEffectiveTemplate(ctx, messageTypeKey, defaultJSON)
	if err != nil {
		return err
	}

	rendered := template.Render(tmpl, d.mergeSystemVariables(ctx, vars))
	for _, target := range targets {
		_ = d.dispatchToTarget(ctx, messageTypeKey, target, rendered)
	}

	if err := d.dispatchCategoryDMs(ctx, messageTypeKey, category, rendered); err != nil {
		return err
	}
	if messageTypeKey == "player_joined" || messageTypeKey == "player_left" {
		_ = d.dispatchPersonalPlayerDMs(ctx, messageTypeKey, vars, rendered)
	}
	return nil
}

// NotifyPlayerAutoLinked sends a DM when pending_player_name is resolved to player_id.
func (d *Dispatcher) NotifyPlayerAutoLinked(ctx context.Context, links []PlayerAutoLink) error {
	provider, ok := d.directProvider()
	if !ok {
		return nil
	}
	for _, link := range links {
		if strings.TrimSpace(link.ExternalUserID) == "" {
			continue
		}
		msg := RenderedMessage{Plain: fmt.Sprintf(
			"🔗 **Player linked**\n\nYour in-game character **%s** is now linked to your FactoryMate account.",
			link.PlayerName,
		)}
		preview := RedactForLog(msg.Plain)
		sendErr := provider.SendDirect(ctx, "discord", link.ExternalUserID, msg)
		d.recordDMLog(ctx, "player_auto_linked", link.ExternalUserID, preview, sendErr == nil, sendErr)
	}
	return nil
}

// PlayerAutoLink is returned when a pending player name is auto-linked.
type PlayerAutoLink struct {
	ExternalUserID string
	PlayerName     string
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

func (d *Dispatcher) loadMessageType(ctx context.Context, key string) (bool, string, string, error) {
	var enabled bool
	var category, defaultJSON string
	err := d.DB.QueryRowContext(ctx, `
		SELECT enabled, category, default_template_json FROM message_types WHERE key = ?`, key,
	).Scan(&enabled, &category, &defaultJSON)
	if err == sql.ErrNoRows {
		return false, "", "", fmt.Errorf("unknown message type %q", key)
	}
	if err != nil {
		return false, "", "", fmt.Errorf("load message type %q: %w", key, err)
	}
	return enabled, category, defaultJSON, nil
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
		d.recordChannelLog(ctx, messageTypeKey, target.ID, preview, false, err)
		return err
	}

	sendErr := provider.Send(ctx, target, msg)
	d.recordChannelLog(ctx, messageTypeKey, target.ID, preview, sendErr == nil, sendErr)
	return sendErr
}

func (d *Dispatcher) dispatchCategoryDMs(ctx context.Context, messageTypeKey, category string, rendered template.RenderedMessage) error {
	if category == "" || d.Prefs == nil {
		return nil
	}
	provider, ok := d.directProvider()
	if !ok {
		return nil
	}

	recipients, err := d.Prefs.ListDMRecipients(ctx, category)
	if err != nil {
		return fmt.Errorf("list dm recipients for %q: %w", category, err)
	}
	if len(recipients) == 0 {
		return nil
	}

	msg := providerMessage("discord", rendered)
	preview := renderedPreview("discord", rendered)
	for _, recipient := range recipients {
		sendErr := provider.SendDirect(ctx, "discord", recipient.ExternalUserID, msg)
		d.recordDMLog(ctx, messageTypeKey, recipient.ExternalUserID, preview, sendErr == nil, sendErr)
	}
	return nil
}

func (d *Dispatcher) dispatchPersonalPlayerDMs(ctx context.Context, messageTypeKey string, vars map[string]string, rendered template.RenderedMessage) error {
	if d.Prefs == nil {
		return nil
	}
	provider, ok := d.directProvider()
	if !ok {
		return nil
	}

	playerName := strings.TrimSpace(vars["PlayerName"])
	recipients, err := d.Prefs.FindPersonalPlayerRecipients(ctx, playerName)
	if err != nil {
		return fmt.Errorf("personal player recipients: %w", err)
	}
	if len(recipients) == 0 {
		return nil
	}

	personal := personalPlayerMessage(messageTypeKey, rendered)
	preview := notifyRenderedPreview(personal)
	for _, recipient := range recipients {
		sendErr := provider.SendDirect(ctx, "discord", recipient.ExternalUserID, personal)
		d.recordDMLog(ctx, messageTypeKey, recipient.ExternalUserID, preview, sendErr == nil, sendErr)
	}
	return nil
}

func personalPlayerMessage(messageTypeKey string, rendered template.RenderedMessage) RenderedMessage {
	msg := providerMessage("discord", rendered)
	if msg.Embed == nil {
		return msg
	}
	embed := *msg.Embed
	switch messageTypeKey {
	case "player_joined":
		embed.Title = "👤 Your character joined the server"
	case "player_left":
		embed.Title = "👤 Your character disconnected"
	}
	msg.Embed = &embed
	return msg
}

func (d *Dispatcher) directProvider() (DirectMessageProvider, bool) {
	for _, provider := range d.Providers {
		if dm, ok := provider.(DirectMessageProvider); ok {
			return dm, true
		}
	}
	return nil, false
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
	var preview string
	if providerType == "discord" && rendered.Embed != nil {
		parts := make([]string, 0, 2)
		if rendered.Embed.Title != "" {
			parts = append(parts, rendered.Embed.Title)
		}
		if rendered.Embed.Description != "" {
			parts = append(parts, rendered.Embed.Description)
		}
		preview = strings.Join(parts, " — ")
	} else {
		preview = rendered.Plain
	}
	return RedactForLog(preview)
}

func notifyRenderedPreview(msg RenderedMessage) string {
	var preview string
	if msg.Embed != nil {
		parts := make([]string, 0, 2)
		if msg.Embed.Title != "" {
			parts = append(parts, msg.Embed.Title)
		}
		if msg.Embed.Description != "" {
			parts = append(parts, msg.Embed.Description)
		}
		preview = strings.Join(parts, " — ")
	} else {
		preview = msg.Plain
	}
	return RedactForLog(preview)
}

func (d *Dispatcher) recordChannelLog(ctx context.Context, messageTypeKey string, targetID int64, preview string, success bool, sendErr error) {
	var errText sql.NullString
	if sendErr != nil {
		errText = sql.NullString{String: sendErr.Error(), Valid: true}
	}

	sentAt := d.Now().UTC().Format(time.RFC3339)
	_, err := d.DB.ExecContext(ctx, `
		INSERT INTO notification_log (message_type_key, target_id, rendered_preview, success, error, sent_at, delivery_mode)
		VALUES (?, ?, ?, ?, ?, ?, 'channel')`,
		messageTypeKey, targetID, preview, success, errText, sentAt,
	)
	if err != nil {
		_ = err
	}
}

func (d *Dispatcher) recordDMLog(ctx context.Context, messageTypeKey, externalUserID, preview string, success bool, sendErr error) {
	var errText sql.NullString
	if sendErr != nil {
		errText = sql.NullString{String: sendErr.Error(), Valid: true}
	}

	sentAt := d.Now().UTC().Format(time.RFC3339)
	_, err := d.DB.ExecContext(ctx, `
		INSERT INTO notification_log (message_type_key, target_id, rendered_preview, success, error, sent_at, delivery_mode, recipient_external_user_id)
		VALUES (?, NULL, ?, ?, ?, ?, 'dm', ?)`,
		messageTypeKey, preview, success, errText, sentAt, externalUserID,
	)
	if err != nil {
		_ = err
	}
}
