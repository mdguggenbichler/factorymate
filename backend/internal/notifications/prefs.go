package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	KeyDMDefaultsJSON          = "notifications.dm_defaults_json"
	KeyDMPlayerPersonalDefault = "notifications.dm_player_personal_default"
)

// Category identifiers for DM opt-in (discord-bot-plan §9.3).
const (
	CategoryServer      = "server"
	CategoryPlayer      = "player"
	CategoryPower       = "power"
	CategoryProgression = "progression"
	CategoryVehicle     = "vehicle"
)

// AllCategories lists DM preference categories in display order.
var AllCategories = []string{
	CategoryServer,
	CategoryPlayer,
	CategoryPower,
	CategoryProgression,
	CategoryVehicle,
}

// UserPrefs is the per-user notification preference payload.
type UserPrefs struct {
	Categories         map[string]bool `json:"categories"`
	DMPlayerPersonal   bool            `json:"dmPlayerPersonal"`
}

// AdminDefaults configures inherited prefs for new users.
type AdminDefaults struct {
	Categories               map[string]bool `json:"categories"`
	DMPlayerPersonalDefault  bool            `json:"dmPlayerPersonalDefault"`
}

// Service manages notification preference storage.
type Service struct {
	DB  *sql.DB
	Now func() time.Time
}

// NewService constructs a notification preferences service.
func NewService(db *sql.DB) *Service {
	return &Service{DB: db, Now: time.Now}
}

func defaultCategoryMap() map[string]bool {
	out := make(map[string]bool, len(AllCategories))
	for _, c := range AllCategories {
		out[c] = false
	}
	return out
}

// GetAdminDefaults returns DM defaults for new users.
func (s *Service) GetAdminDefaults(ctx context.Context) (AdminDefaults, error) {
	return getAdminDefaults(ctx, s.DB)
}

func getAdminDefaults(ctx context.Context, querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (AdminDefaults, error) {
	defaults := AdminDefaults{Categories: defaultCategoryMap()}

	raw, err := querySetting(ctx, querier, KeyDMDefaultsJSON)
	if err != nil {
		return AdminDefaults{}, err
	}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &defaults.Categories); err != nil {
			return AdminDefaults{}, fmt.Errorf("parse dm defaults: %w", err)
		}
	}
	for _, c := range AllCategories {
		if _, ok := defaults.Categories[c]; !ok {
			defaults.Categories[c] = false
		}
	}

	personalRaw, err := querySetting(ctx, querier, KeyDMPlayerPersonalDefault)
	if err != nil {
		return AdminDefaults{}, err
	}
	defaults.DMPlayerPersonalDefault = personalRaw == "true"
	return defaults, nil
}

// SetAdminDefaults updates DM defaults for new users.
func (s *Service) SetAdminDefaults(ctx context.Context, input AdminDefaults) (AdminDefaults, error) {
	categories := defaultCategoryMap()
	for _, c := range AllCategories {
		if v, ok := input.Categories[c]; ok {
			categories[c] = v
		}
	}
	body, err := json.Marshal(categories)
	if err != nil {
		return AdminDefaults{}, fmt.Errorf("marshal dm defaults: %w", err)
	}
	if err := setSetting(ctx, s.DB, KeyDMDefaultsJSON, string(body)); err != nil {
		return AdminDefaults{}, err
	}
	personal := "false"
	if input.DMPlayerPersonalDefault {
		personal = "true"
	}
	if err := setSetting(ctx, s.DB, KeyDMPlayerPersonalDefault, personal); err != nil {
		return AdminDefaults{}, err
	}
	return s.GetAdminDefaults(ctx)
}

// GetUserPrefs returns effective prefs for a user (missing rows use admin defaults).
func (s *Service) GetUserPrefs(ctx context.Context, userID int64) (UserPrefs, error) {
	defaults, err := s.GetAdminDefaults(ctx)
	if err != nil {
		return UserPrefs{}, err
	}

	prefs := UserPrefs{
		Categories:       copyCategoryMap(defaults.Categories),
		DMPlayerPersonal: defaults.DMPlayerPersonalDefault,
	}

	rows, err := s.DB.QueryContext(ctx, `
		SELECT category, dm_enabled FROM user_notification_prefs WHERE user_id = ?`, userID,
	)
	if err != nil {
		return UserPrefs{}, fmt.Errorf("query user prefs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var category string
		var enabled bool
		if err := rows.Scan(&category, &enabled); err != nil {
			return UserPrefs{}, fmt.Errorf("scan user pref: %w", err)
		}
		prefs.Categories[category] = enabled
	}
	if err := rows.Err(); err != nil {
		return UserPrefs{}, err
	}

	var personal sql.NullBool
	err = s.DB.QueryRowContext(ctx, `SELECT dm_player_personal FROM users WHERE id = ?`, userID).Scan(&personal)
	if err == sql.ErrNoRows {
		return UserPrefs{}, fmt.Errorf("user not found")
	}
	if err != nil {
		return UserPrefs{}, fmt.Errorf("query dm_player_personal: %w", err)
	}
	if personal.Valid {
		prefs.DMPlayerPersonal = personal.Bool
	}
	return prefs, nil
}

// SetUserPrefs updates a user's DM preferences.
func (s *Service) SetUserPrefs(ctx context.Context, userID int64, input UserPrefs) (UserPrefs, error) {
	var exists int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ?`, userID).Scan(&exists); err != nil {
		return UserPrefs{}, fmt.Errorf("check user: %w", err)
	}
	if exists == 0 {
		return UserPrefs{}, fmt.Errorf("user not found")
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return UserPrefs{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := s.Now().UTC().Format(time.RFC3339)
	for _, category := range AllCategories {
		enabled, ok := input.Categories[category]
		if !ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_notification_prefs (user_id, category, dm_enabled, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(user_id, category) DO UPDATE SET
				dm_enabled = excluded.dm_enabled,
				updated_at = excluded.updated_at`,
			userID, category, enabled, now,
		); err != nil {
			return UserPrefs{}, fmt.Errorf("upsert pref %s: %w", category, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE users SET dm_player_personal = ? WHERE id = ?`, input.DMPlayerPersonal, userID); err != nil {
		return UserPrefs{}, fmt.Errorf("update dm_player_personal: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return UserPrefs{}, fmt.Errorf("commit tx: %w", err)
	}
	return s.GetUserPrefs(ctx, userID)
}

// SeedUserPrefs inserts category rows for a new user from admin defaults.
func (s *Service) SeedUserPrefs(ctx context.Context, tx *sql.Tx, userID int64) error {
	defaults, err := getAdminDefaults(ctx, tx)
	if err != nil {
		return err
	}
	now := s.Now().UTC().Format(time.RFC3339)
	for _, category := range AllCategories {
		enabled := defaults.Categories[category]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_notification_prefs (user_id, category, dm_enabled, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(user_id, category) DO NOTHING`,
			userID, category, enabled, now,
		); err != nil {
			return fmt.Errorf("seed pref %s: %w", category, err)
		}
	}
	personal := 0
	if defaults.DMPlayerPersonalDefault {
		personal = 1
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET dm_player_personal = ? WHERE id = ?`, personal, userID); err != nil {
		return fmt.Errorf("seed dm_player_personal: %w", err)
	}
	return nil
}

// DMRecipient is an active linked user eligible for category DMs.
type DMRecipient struct {
	UserID         int64
	ExternalUserID string
}

// ListDMRecipients returns users opted in to category DMs.
func (s *Service) ListDMRecipients(ctx context.Context, category string) ([]DMRecipient, error) {
	defaults, err := s.GetAdminDefaults(ctx)
	if err != nil {
		return nil, err
	}
	defaultEnabled := defaults.Categories[category]

	rows, err := s.DB.QueryContext(ctx, `
		SELECT u.id, u.external_user_id, COALESCE(unp.dm_enabled, ?) AS dm_enabled
		FROM users u
		LEFT JOIN user_notification_prefs unp
			ON unp.user_id = u.id AND unp.category = ?
		WHERE u.status = 'active'
			AND u.external_user_id IS NOT NULL
			AND u.external_platform = 'discord'`,
		defaultEnabled, category,
	)
	if err != nil {
		return nil, fmt.Errorf("query dm recipients: %w", err)
	}
	defer rows.Close()

	var out []DMRecipient
	for rows.Next() {
		var userID int64
		var externalUserID string
		var enabled bool
		if err := rows.Scan(&userID, &externalUserID, &enabled); err != nil {
			return nil, fmt.Errorf("scan dm recipient: %w", err)
		}
		if !enabled || strings.TrimSpace(externalUserID) == "" {
			continue
		}
		out = append(out, DMRecipient{UserID: userID, ExternalUserID: externalUserID})
	}
	return out, rows.Err()
}

// PersonalPlayerRecipient matches a player event to a user with personal DMs enabled.
type PersonalPlayerRecipient struct {
	UserID         int64
	ExternalUserID string
}

// FindPersonalPlayerRecipients returns users who should receive a personal player-event DM.
func (s *Service) FindPersonalPlayerRecipients(ctx context.Context, playerName string) ([]PersonalPlayerRecipient, error) {
	playerName = strings.TrimSpace(playerName)
	if playerName == "" {
		return nil, nil
	}

	rows, err := s.DB.QueryContext(ctx, `
		SELECT u.id, u.external_user_id
		FROM users u
		LEFT JOIN player_state p ON p.player_id = u.player_id
		WHERE u.status = 'active'
			AND u.external_user_id IS NOT NULL
			AND u.external_platform = 'discord'
			AND u.dm_player_personal = 1
			AND (
				(u.player_id IS NOT NULL AND LOWER(p.name) = LOWER(?))
				OR (u.pending_player_name IS NOT NULL AND LOWER(u.pending_player_name) = LOWER(?))
			)`,
		playerName, playerName,
	)
	if err != nil {
		return nil, fmt.Errorf("query personal recipients: %w", err)
	}
	defer rows.Close()

	var out []PersonalPlayerRecipient
	for rows.Next() {
		var r PersonalPlayerRecipient
		if err := rows.Scan(&r.UserID, &r.ExternalUserID); err != nil {
			return nil, fmt.Errorf("scan personal recipient: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func getSetting(ctx context.Context, db *sql.DB, key string) (string, error) {
	return querySetting(ctx, db, key)
}

func querySetting(ctx context.Context, querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, key string) (string, error) {
	var value string
	err := querier.QueryRowContext(ctx, `SELECT value FROM app_setting_kv WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

func setSetting(ctx context.Context, db *sql.DB, key, value string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

func copyCategoryMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
