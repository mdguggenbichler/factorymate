package connection

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"factorymate/internal/auth"
	"factorymate/internal/notify"
	"factorymate/internal/registration"
)

const (
	settingKeyDetails    = "connection.details_json"
	settingSMMProfileName = "mods.smm_profile_name"
	defaultSMMProfileName = "FactoryMate Server"
	messageTypeKey       = "connection_details_changed"
	dmRateLimit          = 200 * time.Millisecond
)

// DirectMessenger delivers outbound DMs (notify.DiscordProvider).
type DirectMessenger interface {
	SendDirect(ctx context.Context, platform, externalUserID string, msg notify.RenderedMessage) error
}

// Service manages game join connection details (§8).
type Service struct {
	DB     *sql.DB
	SendDM DirectMessenger
	Now    func() time.Time
}

// NewService constructs a connection details service.
func NewService(db *sql.DB, dm DirectMessenger) *Service {
	return &Service{
		DB:     db,
		SendDM: dm,
		Now:    time.Now,
	}
}

// Get returns stored connection details.
func (s *Service) Get(ctx context.Context) (Details, error) {
	profileName, err := s.getSMMProfileName(ctx)
	if err != nil {
		return Details{}, err
	}

	var raw string
	err = s.DB.QueryRowContext(ctx, `
		SELECT value FROM app_setting_kv WHERE key = ?`, settingKeyDetails,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return Details{SMMProfileName: profileName}, nil
	}
	if err != nil {
		return Details{}, fmt.Errorf("get connection details: %w", err)
	}
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return Details{SMMProfileName: profileName}, nil
	}
	var d Details
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return Details{}, fmt.Errorf("parse connection details: %w", err)
	}
	d.SMMProfileName = profileName
	return d, nil
}

// Set updates connection details and broadcasts DMs to active linked users.
func (s *Service) Set(ctx context.Context, input UpdateInput, updatedByUserID int64) (Details, error) {
	old, err := s.Get(ctx)
	if err != nil {
		return Details{}, err
	}

	merged := mergeDetails(old, input)
	if strings.TrimSpace(merged.GameHost) == "" {
		return Details{}, fmt.Errorf("gameHost is required")
	}
	if merged.GamePort <= 0 {
		return Details{}, fmt.Errorf("gamePort must be positive")
	}

	now := s.Now().UTC().Format(time.RFC3339)
	merged.UpdatedAt = now
	if updatedByUserID > 0 {
		merged.UpdatedByUserID = &updatedByUserID
	}

	toSave := merged
	toSave.SMMProfileName = ""
	raw, err := json.Marshal(toSave)
	if err != nil {
		return Details{}, fmt.Errorf("marshal connection details: %w", err)
	}

	if _, err := s.DB.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		settingKeyDetails, string(raw),
	); err != nil {
		return Details{}, fmt.Errorf("save connection details: %w", err)
	}

	if input.SMMProfileName != nil {
		if err := s.setSMMProfileName(ctx, *input.SMMProfileName); err != nil {
			return Details{}, err
		}
	}

	profileName, err := s.getSMMProfileName(ctx)
	if err != nil {
		return Details{}, err
	}
	merged.SMMProfileName = profileName

	if s.SendDM != nil {
		_ = s.BroadcastChange(ctx, old, merged)
	}

	return merged, nil
}

func (s *Service) getSMMProfileName(ctx context.Context) (string, error) {
	var name string
	err := s.DB.QueryRowContext(ctx, `
		SELECT value FROM app_setting_kv WHERE key = ?`, settingSMMProfileName,
	).Scan(&name)
	if err == sql.ErrNoRows || strings.TrimSpace(name) == "" {
		return defaultSMMProfileName, nil
	}
	if err != nil {
		return defaultSMMProfileName, fmt.Errorf("get smm profile name: %w", err)
	}
	return strings.TrimSpace(name), nil
}

func (s *Service) setSMMProfileName(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultSMMProfileName
	}
	if _, err := s.DB.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		settingSMMProfileName, name,
	); err != nil {
		return fmt.Errorf("save smm profile name: %w", err)
	}
	return nil
}

func mergeDetails(old Details, input UpdateInput) Details {
	out := old
	if input.GameHost != nil {
		out.GameHost = strings.TrimSpace(*input.GameHost)
	}
	if input.GamePort != nil {
		out.GamePort = *input.GamePort
	}
	if input.ClearPassword {
		out.GamePassword = ""
	} else if input.GamePassword != nil {
		out.GamePassword = *input.GamePassword
	}
	if input.Notes != nil {
		out.Notes = strings.TrimSpace(*input.Notes)
	}
	return out
}

// BroadcastChange sends mandatory connection-detail DMs to all active linked users (§8.4).
func (s *Service) BroadcastChange(ctx context.Context, old, new Details) error {
	recipients, err := s.listActiveLinkedUsers(ctx)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return nil
	}

	msg := FormatChangeDM(new, old)
	preview := RedactForLog(msg.Plain)

	for i, extID := range recipients {
		sendErr := s.SendDM.SendDirect(ctx, registration.PlatformDiscord, extID, msg)
		s.recordDMLog(ctx, extID, preview, sendErr == nil, sendErr)
		if i < len(recipients)-1 {
			time.Sleep(dmRateLimit)
		}
	}
	return nil
}

// SendToUser delivers current connection details to one external user.
func (s *Service) SendToUser(ctx context.Context, externalUserID string) error {
	details, err := s.Get(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(details.GameHost) == "" {
		return fmt.Errorf("connection details are not configured")
	}
	msg := FormatDetailsDM(details)
	sendErr := s.SendDM.SendDirect(ctx, registration.PlatformDiscord, externalUserID, msg)
	s.recordDMLog(ctx, externalUserID, RedactForLog(msg.Plain), sendErr == nil, sendErr)
	return sendErr
}

func (s *Service) listActiveLinkedUsers(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT external_user_id FROM users
		WHERE status = ? AND external_user_id IS NOT NULL`,
		auth.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("query linked users: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var extID string
		if err := rows.Scan(&extID); err != nil {
			return nil, fmt.Errorf("scan external user id: %w", err)
		}
		if strings.TrimSpace(extID) != "" {
			out = append(out, extID)
		}
	}
	return out, rows.Err()
}

func (s *Service) recordDMLog(ctx context.Context, externalUserID, preview string, success bool, sendErr error) {
	var errText sql.NullString
	if sendErr != nil {
		errText = sql.NullString{String: sendErr.Error(), Valid: true}
	}
	sentAt := s.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO notification_log (
			message_type_key, target_id, rendered_preview, success, error, sent_at,
			delivery_mode, recipient_external_user_id
		) VALUES (?, NULL, ?, ?, ?, ?, 'dm', ?)`,
		messageTypeKey, preview, success, errText, sentAt, externalUserID,
	)
	if err != nil {
		_ = err
	}
}

// FormatDetailsDM builds the user-facing connection message (Appendix A).
func FormatDetailsDM(d Details) notify.RenderedMessage {
	return notify.RenderedMessage{Plain: formatDetailsPlain(d, false)}
}

// FormatChangeDM builds a broadcast message when details change.
func FormatChangeDM(new, old Details) notify.RenderedMessage {
	header := "🔔 **Connection details updated**\n\n"
	body := formatDetailsPlain(new, true)
	if old.GameHost != new.GameHost || old.GamePort != new.GamePort {
		// diff summary already in header
	} else if old.GamePassword != new.GamePassword {
		header += "Password was changed.\n\n"
	}
	return notify.RenderedMessage{Plain: header + body}
}

func formatDetailsPlain(d Details, includeHeader bool) string {
	var lines []string
	if includeHeader {
		lines = append(lines, "🏭 Server Connection")
		lines = append(lines, "")
	}
	lines = append(lines, fmt.Sprintf("Host:     %s", d.GameHost))
	lines = append(lines, fmt.Sprintf("Port:     %d", d.GamePort))
	if d.GamePassword != "" {
		lines = append(lines, fmt.Sprintf("Password: ||%s||", d.GamePassword))
	}
	if strings.TrimSpace(d.Notes) != "" {
		lines = append(lines, fmt.Sprintf("Notes:    %s", d.Notes))
	}
	if d.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, d.UpdatedAt); err == nil {
			lines = append(lines, fmt.Sprintf("Updated:  %s", t.UTC().Format("Jan 2, 2006 · 15:04 UTC")))
		}
	}
	return strings.Join(lines, "\n")
}
