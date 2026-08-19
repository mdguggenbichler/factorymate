package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrUserNotFound is returned when a user id does not exist.
var ErrUserNotFound = errors.New("user not found")

const (
	KeyDMDefaultsJSON          = "notifications.dm_defaults_json"
	KeyDMPlayerPersonalDefault = "notifications.dm_player_personal_default"
)

// Category identifiers for grouping (Discord coarse control + UI headers).
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

var excludedPrefKeys = map[string]struct{}{
	"connection_details":         {},
	"connection_details_changed": {},
}

// ChannelTarget is a safe catalog hint for overlap copy (no Discord IDs required).
type ChannelTarget struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CatalogEntry is a viewer-safe message-type row for notification preference UIs.
type CatalogEntry struct {
	Key             string          `json:"key"`
	Label           string          `json:"label"`
	Category        string          `json:"category"`
	GloballyEnabled bool            `json:"globallyEnabled"`
	ChannelTargets  []ChannelTarget `json:"channelTargets"`
}

// UserPrefs is the per-user notification preference payload.
type UserPrefs struct {
	Types            map[string]bool `json:"types"`
	DMPlayerPersonal bool            `json:"dmPlayerPersonal"`
	Catalog          []CatalogEntry  `json:"catalog,omitempty"`
}

// UserPrefsPatch is a partial update for PUT /api/account/notifications.
type UserPrefsPatch struct {
	Types            map[string]bool `json:"types"`
	DMPlayerPersonal *bool           `json:"dmPlayerPersonal"`
}

// AdminDefaults configures inherited prefs for new users.
type AdminDefaults struct {
	Types                   map[string]bool `json:"types"`
	DMPlayerPersonalDefault bool            `json:"dmPlayerPersonalDefault"`
	Catalog                 []CatalogEntry  `json:"catalog,omitempty"`
}

// AdminDefaultsPatch is a partial update for PUT /api/settings/notification-defaults.
type AdminDefaultsPatch struct {
	Types                   map[string]bool `json:"types"`
	DMPlayerPersonalDefault *bool           `json:"dmPlayerPersonalDefault"`
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

// PrefTypeExcluded reports whether a message type is omitted from personal DM prefs.
func PrefTypeExcluded(key string) bool {
	_, ok := excludedPrefKeys[key]
	return ok
}

// GetAdminDefaults returns DM defaults for new users.
func (s *Service) GetAdminDefaults(ctx context.Context) (AdminDefaults, error) {
	catalog, err := s.LoadCatalog(ctx)
	if err != nil {
		return AdminDefaults{}, err
	}
	return getAdminDefaults(ctx, s.DB, catalog)
}

func getAdminDefaults(ctx context.Context, querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, catalog []CatalogEntry) (AdminDefaults, error) {
	defaults := AdminDefaults{Types: defaultTypeMap(catalog), Catalog: catalog}

	raw, err := querySetting(ctx, querier, KeyDMDefaultsJSON)
	if err != nil {
		return AdminDefaults{}, err
	}
	if strings.TrimSpace(raw) != "" {
		parsed := map[string]bool{}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return AdminDefaults{}, fmt.Errorf("parse dm defaults: %w", err)
		}
		defaults.Types = mergeDefaultsJSON(parsed, catalog)
	}

	personalRaw, err := querySetting(ctx, querier, KeyDMPlayerPersonalDefault)
	if err != nil {
		return AdminDefaults{}, err
	}
	defaults.DMPlayerPersonalDefault = personalRaw == "true"
	return defaults, nil
}

// SetAdminDefaults updates DM defaults for new users (partial).
func (s *Service) SetAdminDefaults(ctx context.Context, input AdminDefaultsPatch) (AdminDefaults, error) {
	current, err := s.GetAdminDefaults(ctx)
	if err != nil {
		return AdminDefaults{}, err
	}
	types := copyBoolMap(current.Types)
	for key, enabled := range input.Types {
		if !isCatalogKey(current.Catalog, key) {
			continue
		}
		types[key] = enabled
	}
	body, err := json.Marshal(types)
	if err != nil {
		return AdminDefaults{}, fmt.Errorf("marshal dm defaults: %w", err)
	}
	if err := setSetting(ctx, s.DB, KeyDMDefaultsJSON, string(body)); err != nil {
		return AdminDefaults{}, err
	}
	if input.DMPlayerPersonalDefault != nil {
		personal := "false"
		if *input.DMPlayerPersonalDefault {
			personal = "true"
		}
		if err := setSetting(ctx, s.DB, KeyDMPlayerPersonalDefault, personal); err != nil {
			return AdminDefaults{}, err
		}
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
		Types:            copyBoolMap(defaults.Types),
		DMPlayerPersonal: defaults.DMPlayerPersonalDefault,
		Catalog:          defaults.Catalog,
	}

	rows, err := s.DB.QueryContext(ctx, `
		SELECT message_type_key, dm_enabled FROM user_notification_prefs WHERE user_id = ?`, userID,
	)
	if err != nil {
		return UserPrefs{}, fmt.Errorf("query user prefs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var enabled bool
		if err := rows.Scan(&key, &enabled); err != nil {
			return UserPrefs{}, fmt.Errorf("scan user pref: %w", err)
		}
		if PrefTypeExcluded(key) {
			continue
		}
		prefs.Types[key] = enabled
	}
	if err := rows.Err(); err != nil {
		return UserPrefs{}, err
	}

	var personal sql.NullBool
	err = s.DB.QueryRowContext(ctx, `SELECT dm_player_personal FROM users WHERE id = ?`, userID).Scan(&personal)
	if err == sql.ErrNoRows {
		return UserPrefs{}, ErrUserNotFound
	}
	if err != nil {
		return UserPrefs{}, fmt.Errorf("query dm_player_personal: %w", err)
	}
	if personal.Valid {
		prefs.DMPlayerPersonal = personal.Bool
	}
	return prefs, nil
}

// SetUserPrefs updates a user's DM preferences (partial type map and optional personal toggle).
func (s *Service) SetUserPrefs(ctx context.Context, userID int64, input UserPrefsPatch) (UserPrefs, error) {
	var exists int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ?`, userID).Scan(&exists); err != nil {
		return UserPrefs{}, fmt.Errorf("check user: %w", err)
	}
	if exists == 0 {
		return UserPrefs{}, ErrUserNotFound
	}

	catalog, err := s.LoadCatalog(ctx)
	if err != nil {
		return UserPrefs{}, err
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return UserPrefs{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := s.Now().UTC().Format(time.RFC3339)
	for key, enabled := range input.Types {
		if !isCatalogKey(catalog, key) {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_notification_prefs (user_id, message_type_key, dm_enabled, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(user_id, message_type_key) DO UPDATE SET
				dm_enabled = excluded.dm_enabled,
				updated_at = excluded.updated_at`,
			userID, key, enabled, now,
		); err != nil {
			return UserPrefs{}, fmt.Errorf("upsert pref %s: %w", key, err)
		}
	}

	if input.DMPlayerPersonal != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET dm_player_personal = ? WHERE id = ?`, *input.DMPlayerPersonal, userID); err != nil {
			return UserPrefs{}, fmt.Errorf("update dm_player_personal: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return UserPrefs{}, fmt.Errorf("commit tx: %w", err)
	}
	return s.GetUserPrefs(ctx, userID)
}

// SeedUserPrefs inserts per-type rows for a new user from admin defaults.
func (s *Service) SeedUserPrefs(ctx context.Context, tx *sql.Tx, userID int64) error {
	catalog, err := loadCatalog(ctx, tx)
	if err != nil {
		return err
	}
	defaults, err := getAdminDefaults(ctx, tx, catalog)
	if err != nil {
		return err
	}
	now := s.Now().UTC().Format(time.RFC3339)
	for _, entry := range catalog {
		enabled := defaults.Types[entry.Key]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_notification_prefs (user_id, message_type_key, dm_enabled, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(user_id, message_type_key) DO NOTHING`,
			userID, entry.Key, enabled, now,
		); err != nil {
			return fmt.Errorf("seed pref %s: %w", entry.Key, err)
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

// DMRecipient is an active linked user eligible for type DMs.
type DMRecipient struct {
	UserID         int64
	ExternalUserID string
}

// ListDMRecipients returns users opted in to DMs for a message type key.
func (s *Service) ListDMRecipients(ctx context.Context, messageTypeKey string) ([]DMRecipient, error) {
	if PrefTypeExcluded(messageTypeKey) {
		return nil, nil
	}
	defaults, err := s.GetAdminDefaults(ctx)
	if err != nil {
		return nil, err
	}
	if !isCatalogKey(defaults.Catalog, messageTypeKey) {
		return nil, nil
	}
	defaultEnabled := defaults.Types[messageTypeKey]

	rows, err := s.DB.QueryContext(ctx, `
		SELECT u.id, u.external_user_id, COALESCE(unp.dm_enabled, ?) AS dm_enabled
		FROM users u
		LEFT JOIN user_notification_prefs unp
			ON unp.user_id = u.id AND unp.message_type_key = ?
		WHERE u.status = 'active'
			AND u.external_user_id IS NOT NULL
			AND u.external_platform = 'discord'`,
		defaultEnabled, messageTypeKey,
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

// TypeKeysInCategory returns catalog keys for a category (Discord coarse control).
func (s *Service) TypeKeysInCategory(ctx context.Context, category string) ([]string, error) {
	catalog, err := s.LoadCatalog(ctx)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, entry := range catalog {
		if entry.Category == category {
			keys = append(keys, entry.Key)
		}
	}
	return keys, nil
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

// LoadCatalog returns viewer-safe pref catalog rows.
func (s *Service) LoadCatalog(ctx context.Context) ([]CatalogEntry, error) {
	return loadCatalog(ctx, s.DB)
}

type catalogQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadCatalog(ctx context.Context, querier catalogQuerier) ([]CatalogEntry, error) {
	rows, err := querier.QueryContext(ctx, `
		SELECT key, label, category, enabled
		FROM message_types
		WHERE key NOT IN ('connection_details', 'connection_details_changed')
		ORDER BY CASE category
			WHEN 'server' THEN 1
			WHEN 'player' THEN 2
			WHEN 'power' THEN 3
			WHEN 'progression' THEN 4
			WHEN 'vehicle' THEN 5
			ELSE 6
		END, key`,
	)
	if err != nil {
		return nil, fmt.Errorf("query pref catalog: %w", err)
	}
	defer rows.Close()

	var catalog []CatalogEntry
	index := map[string]int{}
	for rows.Next() {
		var entry CatalogEntry
		if err := rows.Scan(&entry.Key, &entry.Label, &entry.Category, &entry.GloballyEnabled); err != nil {
			return nil, fmt.Errorf("scan pref catalog: %w", err)
		}
		entry.ChannelTargets = []ChannelTarget{}
		index[entry.Key] = len(catalog)
		catalog = append(catalog, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	targetRows, err := querier.QueryContext(ctx, `
		SELECT mtt.message_type_key, nt.id, nt.name
		FROM message_type_targets mtt
		JOIN notification_targets nt ON nt.id = mtt.target_id
		WHERE nt.enabled = 1
		ORDER BY nt.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query catalog targets: %w", err)
	}
	defer targetRows.Close()

	for targetRows.Next() {
		var key, name string
		var id int64
		if err := targetRows.Scan(&key, &id, &name); err != nil {
			return nil, fmt.Errorf("scan catalog target: %w", err)
		}
		i, ok := index[key]
		if !ok {
			continue
		}
		catalog[i].ChannelTargets = append(catalog[i].ChannelTargets, ChannelTarget{ID: id, Name: name})
	}
	return catalog, targetRows.Err()
}

// CategorySummary describes coarse Discord view state for one category.
func CategorySummary(types map[string]bool, catalog []CatalogEntry, category string) (enabled, total int) {
	for _, entry := range catalog {
		if entry.Category != category {
			continue
		}
		total++
		if types[entry.Key] {
			enabled++
		}
	}
	return enabled, total
}

// CategoryOverlapTargetNames returns unique enabled target names for types in a category.
func CategoryOverlapTargetNames(catalog []CatalogEntry, category string) []string {
	seen := map[string]struct{}{}
	var names []string
	for _, entry := range catalog {
		if entry.Category != category {
			continue
		}
		for _, target := range entry.ChannelTargets {
			if _, ok := seen[target.Name]; ok {
				continue
			}
			seen[target.Name] = struct{}{}
			names = append(names, target.Name)
		}
	}
	return names
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

func defaultTypeMap(catalog []CatalogEntry) map[string]bool {
	out := make(map[string]bool, len(catalog))
	for _, entry := range catalog {
		out[entry.Key] = false
	}
	return out
}

func mergeDefaultsJSON(raw map[string]bool, catalog []CatalogEntry) map[string]bool {
	out := defaultTypeMap(catalog)
	perType := false
	for _, entry := range catalog {
		if _, ok := raw[entry.Key]; ok {
			perType = true
			break
		}
	}
	if perType {
		for _, entry := range catalog {
			if v, ok := raw[entry.Key]; ok {
				out[entry.Key] = v
			}
		}
		return out
	}
	for _, entry := range catalog {
		if v, ok := raw[entry.Category]; ok {
			out[entry.Key] = v
		}
	}
	return out
}

func isCatalogKey(catalog []CatalogEntry, key string) bool {
	if PrefTypeExcluded(key) {
		return false
	}
	for _, entry := range catalog {
		if entry.Key == key {
			return true
		}
	}
	return false
}

func copyBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
