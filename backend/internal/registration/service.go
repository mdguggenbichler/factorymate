package registration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"factorymate/internal/auth"
	"factorymate/internal/notifications"
)

// ExternalIdentity is a chat-platform user reference stored on users.
type ExternalIdentity struct {
	Platform    string
	UserID      string
	Username    string
	DisplayName string
}

// RegisterParams holds inputs for a new Discord registration.
type RegisterParams struct {
	Username          string
	Password          string
	PendingPlayerName string
	External          ExternalIdentity
	Role              auth.Role
	ForceApprove      bool
}

// RegisterResult is returned after a successful registration.
type RegisterResult struct {
	User            auth.User
	PlayerLinked    bool
	PendingApproval bool
}

// PendingRegistration is a user awaiting admin approval.
type PendingRegistration struct {
	ID                int64  `json:"id"`
	Username          string `json:"username"`
	PendingPlayerName string `json:"pendingPlayerName"`
	ExternalUsername  string `json:"externalUsername"`
	ExternalDisplay   string `json:"externalDisplayName"`
	CreatedAt         string `json:"createdAt"`
}

// UnmappedPlayer is a server player with no linked FM user.
type UnmappedPlayer struct {
	PlayerID   string `json:"playerId"`
	Name       string `json:"name"`
	Online     bool   `json:"online"`
	LastSeenAt string `json:"lastSeenAt,omitempty"`
}

// ExternalUpdate is an admin override for external identity fields.
type ExternalUpdate struct {
	Platform    *string
	UserID      *string
	Username    *string
	DisplayName *string
	Unlink      bool
}

// Service handles registration flows shared by REST and Discord.
type Service struct {
	db   *sql.DB
	auth *auth.Service
}

// NewService constructs a registration service.
func NewService(db *sql.DB, authSvc *auth.Service) *Service {
	return &Service{db: db, auth: authSvc}
}

// DB exposes the database for tests.
func (s *Service) DB() *sql.DB {
	return s.db
}

const keyAutoApprove = "registration.auto_approve"

// AutoApproveEnabled reports registration.auto_approve (default true).
func (s *Service) AutoApproveEnabled(ctx context.Context) (bool, error) {
	raw, err := getSetting(ctx, s.db, keyAutoApprove)
	if err != nil {
		return false, err
	}
	if raw == "" {
		return true, nil
	}
	return raw == "true", nil
}

// SetAutoApprove updates registration.auto_approve.
func (s *Service) SetAutoApprove(ctx context.Context, enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	return setSetting(ctx, s.db, keyAutoApprove, val)
}

// GetByExternal returns the user linked to an external identity, or nil.
func (s *Service) GetByExternal(ctx context.Context, platform, externalUserID string) (*auth.User, error) {
	platform = strings.TrimSpace(platform)
	externalUserID = strings.TrimSpace(externalUserID)
	if platform == "" || externalUserID == "" {
		return nil, nil
	}

	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM users
		WHERE external_platform = ? AND external_user_id = ?`,
		platform, externalUserID,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query external user: %w", err)
	}
	user, err := s.auth.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Register creates a new user from Discord onboarding.
func (s *Service) Register(ctx context.Context, params RegisterParams) (RegisterResult, error) {
	username := strings.TrimSpace(params.Username)
	if username == "" {
		return RegisterResult{}, fmt.Errorf("username is required")
	}
	if err := auth.ValidatePassword(params.Password); err != nil {
		return RegisterResult{}, err
	}
	ext := params.External
	if ext.Platform != PlatformDiscord || strings.TrimSpace(ext.UserID) == "" {
		return RegisterResult{}, ErrInvalidExternal
	}

	existing, err := s.GetByExternal(ctx, ext.Platform, ext.UserID)
	if err != nil {
		return RegisterResult{}, err
	}
	if existing != nil {
		return RegisterResult{}, ErrAlreadyRegistered
	}

	username, err = AllocateUsername(ctx, s.db, username)
	if err != nil {
		return RegisterResult{}, err
	}

	autoApprove, err := s.AutoApproveEnabled(ctx)
	if err != nil {
		return RegisterResult{}, err
	}
	status := auth.StatusActive
	pendingApproval := false
	if !autoApprove && !params.ForceApprove {
		status = auth.StatusPendingApproval
		pendingApproval = true
	}

	role := params.Role
	if role == "" {
		role = auth.RoleViewer
	}

	hash, err := auth.HashPassword(params.Password)
	if err != nil {
		return RegisterResult{}, err
	}

	pendingName := strings.TrimSpace(params.PendingPlayerName)
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO users (
			username, password_hash, role, created_at,
			external_platform, external_user_id, external_username, external_display_name, external_linked_at,
			pending_player_name, registration_source, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		username, hash, string(role), now,
		ext.Platform, ext.UserID, nullIfEmpty(ext.Username), nullIfEmpty(ext.DisplayName), now,
		nullIfEmpty(pendingName), SourceDiscord, status,
	)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("insert user: %w", err)
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return RegisterResult{}, fmt.Errorf("last insert id: %w", err)
	}

	playerLinked := false
	if pendingName != "" {
		var playerID string
		err = tx.QueryRowContext(ctx, `
			SELECT player_id FROM player_state WHERE LOWER(name) = LOWER(?) LIMIT 1`, pendingName,
		).Scan(&playerID)
		if err == nil {
			var linked int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE player_id = ?`, playerID).Scan(&linked); err != nil {
				return RegisterResult{}, fmt.Errorf("check player link: %w", err)
			}
			if linked == 0 {
				if _, err := tx.ExecContext(ctx, `UPDATE users SET player_id = ? WHERE id = ?`, playerID, userID); err != nil {
					return RegisterResult{}, fmt.Errorf("link player: %w", err)
				}
				playerLinked = true
			}
		} else if err != sql.ErrNoRows {
			return RegisterResult{}, fmt.Errorf("match player: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO registration_audit_log (user_id, external_user_id, action, created_at)
		VALUES (?, ?, 'submitted', ?)`, userID, ext.UserID, now); err != nil {
		return RegisterResult{}, fmt.Errorf("audit log: %w", err)
	}

	if err := notifications.NewService(s.db).SeedUserPrefs(ctx, tx, userID); err != nil {
		return RegisterResult{}, fmt.Errorf("seed notification prefs: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return RegisterResult{}, fmt.Errorf("commit tx: %w", err)
	}

	user, err := s.auth.GetUserByID(ctx, userID)
	if err != nil {
		return RegisterResult{}, err
	}
	return RegisterResult{
		User:            user,
		PlayerLinked:    playerLinked,
		PendingApproval: pendingApproval,
	}, nil
}

// LinkAccount attaches an external identity to an existing active FM account.
func (s *Service) LinkAccount(ctx context.Context, username, password string, ext ExternalIdentity) (auth.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return auth.User{}, fmt.Errorf("username is required")
	}
	if ext.Platform != PlatformDiscord || strings.TrimSpace(ext.UserID) == "" {
		return auth.User{}, ErrInvalidExternal
	}

	existingExt, err := s.GetByExternal(ctx, ext.Platform, ext.UserID)
	if err != nil {
		return auth.User{}, err
	}
	if existingExt != nil {
		return auth.User{}, ErrAlreadyRegistered
	}

	user, err := s.auth.Authenticate(ctx, username, password)
	if err != nil {
		return auth.User{}, err
	}
	if user.Status == auth.StatusPendingApproval {
		return auth.User{}, auth.ErrPendingApproval
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET
			external_platform = ?,
			external_user_id = ?,
			external_username = ?,
			external_display_name = ?,
			external_linked_at = ?
		WHERE id = ? AND external_user_id IS NULL`,
		ext.Platform, ext.UserID, nullIfEmpty(ext.Username), nullIfEmpty(ext.DisplayName), now, user.ID,
	)
	if err != nil {
		return auth.User{}, fmt.Errorf("link external: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return auth.User{}, ErrAlreadyRegistered
	}
	return s.auth.GetUserByID(ctx, user.ID)
}

// SetPlayerName updates pending_player_name and tries immediate player match.
func (s *Service) SetPlayerName(ctx context.Context, userID int64, name string) (auth.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return auth.User{}, fmt.Errorf("player name is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM users WHERE id = ?`, userID).Scan(&status); err == sql.ErrNoRows {
		return auth.User{}, ErrUserNotFound
	} else if err != nil {
		return auth.User{}, fmt.Errorf("query user: %w", err)
	}
	if status != auth.StatusActive {
		return auth.User{}, auth.ErrPendingApproval
	}

	if _, err := tx.ExecContext(ctx, `UPDATE users SET pending_player_name = ? WHERE id = ?`, name, userID); err != nil {
		return auth.User{}, fmt.Errorf("update pending name: %w", err)
	}

	var playerID string
	err = tx.QueryRowContext(ctx, `
		SELECT player_id FROM player_state WHERE LOWER(name) = LOWER(?) LIMIT 1`, name,
	).Scan(&playerID)
	if err == nil {
		var owner int64
		err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE player_id = ? AND id != ?`, playerID, userID).Scan(&owner)
		if err == nil {
			return auth.User{}, ErrPlayerAlreadyLinked
		}
		if err != sql.ErrNoRows {
			return auth.User{}, fmt.Errorf("check player owner: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET player_id = ? WHERE id = ?`, playerID, userID); err != nil {
			return auth.User{}, fmt.Errorf("set player_id: %w", err)
		}
	} else if err != sql.ErrNoRows {
		return auth.User{}, fmt.Errorf("match player: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return auth.User{}, fmt.Errorf("commit tx: %w", err)
	}
	return s.auth.GetUserByID(ctx, userID)
}

// ApproveRegistration activates a pending user.
func (s *Service) ApproveRegistration(ctx context.Context, userID, actedByUserID int64) (auth.User, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var externalUserID string
	err = tx.QueryRowContext(ctx, `
		SELECT external_user_id FROM users WHERE id = ? AND status = ?`,
		userID, auth.StatusPendingApproval,
	).Scan(&externalUserID)
	if err == sql.ErrNoRows {
		return auth.User{}, ErrNotPendingApproval
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("query pending user: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE users SET status = ? WHERE id = ? AND status = ?`,
		auth.StatusActive, userID, auth.StatusPendingApproval,
	)
	if err != nil {
		return auth.User{}, fmt.Errorf("approve user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return auth.User{}, ErrNotPendingApproval
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO registration_audit_log (user_id, external_user_id, action, acted_by_user_id, created_at)
		VALUES (?, ?, 'approved', ?, ?)`, userID, externalUserID, nullInt64(actedByUserID), now); err != nil {
		return auth.User{}, fmt.Errorf("audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return auth.User{}, fmt.Errorf("commit tx: %w", err)
	}
	return s.auth.GetUserByID(ctx, userID)
}

// RejectRegistration removes a pending user after audit logging.
func (s *Service) RejectRegistration(ctx context.Context, userID, actedByUserID int64, comment string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	comment = strings.TrimSpace(comment)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var externalUserID string
	err = tx.QueryRowContext(ctx, `
		SELECT external_user_id FROM users WHERE id = ? AND status = ?`,
		userID, auth.StatusPendingApproval,
	).Scan(&externalUserID)
	if err == sql.ErrNoRows {
		return "", ErrNotPendingApproval
	}
	if err != nil {
		return "", fmt.Errorf("query pending user: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO registration_audit_log (user_id, external_user_id, action, acted_by_user_id, comment, created_at)
		VALUES (?, ?, 'rejected', ?, ?, ?)`, userID, externalUserID, nullInt64(actedByUserID), nullIfEmpty(comment), now); err != nil {
		return "", fmt.Errorf("audit log: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ? AND status = ?`, userID, auth.StatusPendingApproval); err != nil {
		return "", fmt.Errorf("delete user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}
	return externalUserID, nil
}

const registrationButtonExpiry = 7 * 24 * time.Hour

// RegistrationSubmittedAt returns when a user submitted their registration request.
func (s *Service) RegistrationSubmittedAt(ctx context.Context, userID int64) (time.Time, error) {
	var createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT created_at FROM registration_audit_log
		WHERE user_id = ? AND action = 'submitted'
		ORDER BY id ASC LIMIT 1`, userID,
	).Scan(&createdAt)
	if err == sql.ErrNoRows {
		var userCreated string
		err = s.db.QueryRowContext(ctx, `SELECT created_at FROM users WHERE id = ?`, userID).Scan(&userCreated)
		if err == sql.ErrNoRows {
			return time.Time{}, ErrUserNotFound
		}
		if err != nil {
			return time.Time{}, fmt.Errorf("query user created_at: %w", err)
		}
		createdAt = userCreated
	} else if err != nil {
		return time.Time{}, fmt.Errorf("query registration submitted: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse submitted at: %w", err)
	}
	return parsed, nil
}

// RegistrationButtonExpired reports whether approval buttons are older than 7 days.
func (s *Service) RegistrationButtonExpired(ctx context.Context, userID int64) (bool, error) {
	submitted, err := s.RegistrationSubmittedAt(ctx, userID)
	if err != nil {
		return false, err
	}
	return time.Since(submitted) > registrationButtonExpiry, nil
}

// ListPending returns users awaiting approval.
func (s *Service) ListPending(ctx context.Context) ([]PendingRegistration, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, COALESCE(pending_player_name, ''), COALESCE(external_username, ''),
			COALESCE(external_display_name, ''), created_at
		FROM users
		WHERE status = ?
		ORDER BY created_at`, auth.StatusPendingApproval,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending: %w", err)
	}
	defer rows.Close()

	out := make([]PendingRegistration, 0)
	for rows.Next() {
		var p PendingRegistration
		if err := rows.Scan(&p.ID, &p.Username, &p.PendingPlayerName, &p.ExternalUsername, &p.ExternalDisplay, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListAdminsWithExternal returns active admins that have a linked external identity.
func (s *Service) ListAdminsWithExternal(ctx context.Context) ([]auth.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM users
		WHERE role = ? AND status = ? AND external_user_id IS NOT NULL`,
		auth.RoleAdmin, auth.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("query admins: %w", err)
	}
	defer rows.Close()

	users := make([]auth.User, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan admin id: %w", err)
		}
		user, err := s.auth.GetUserByID(ctx, id)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// ListUnmappedPlayers returns server players not linked to any FM user.
func (s *Service) ListUnmappedPlayers(ctx context.Context) ([]UnmappedPlayer, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.player_id, p.name, p.online, COALESCE(p.last_seen_at, '')
		FROM player_state p
		LEFT JOIN users u ON u.player_id = p.player_id
		WHERE u.id IS NULL
		ORDER BY p.name`)
	if err != nil {
		return nil, fmt.Errorf("query unmapped players: %w", err)
	}
	defer rows.Close()

	out := make([]UnmappedPlayer, 0)
	for rows.Next() {
		var p UnmappedPlayer
		if err := rows.Scan(&p.PlayerID, &p.Name, &p.Online, &p.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan unmapped: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateExternalIdentity admin-overrides or unlinks external identity.
func (s *Service) UpdateExternalIdentity(ctx context.Context, userID int64, update ExternalUpdate) (auth.User, error) {
	if update.Unlink {
		res, err := s.db.ExecContext(ctx, `
			UPDATE users SET
				external_platform = NULL,
				external_user_id = NULL,
				external_username = NULL,
				external_display_name = NULL,
				external_linked_at = NULL
			WHERE id = ?`, userID)
		if err != nil {
			return auth.User{}, fmt.Errorf("unlink external: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return auth.User{}, ErrUserNotFound
		}
		return s.auth.GetUserByID(ctx, userID)
	}

	platform := update.Platform
	userExtID := update.UserID
	if platform != nil && *platform == "" {
		platform = nil
	}
	if userExtID != nil && *userExtID == "" {
		userExtID = nil
	}
	if platform != nil && userExtID != nil {
		var existingID int64
		err := s.db.QueryRowContext(ctx, `
			SELECT id FROM users WHERE external_platform = ? AND external_user_id = ? AND id != ?`,
			*platform, *userExtID, userID,
		).Scan(&existingID)
		if err == nil {
			return auth.User{}, ErrAlreadyRegistered
		}
		if err != sql.ErrNoRows {
			return auth.User{}, fmt.Errorf("check external unique: %w", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET
			external_platform = COALESCE(?, external_platform),
			external_user_id = COALESCE(?, external_user_id),
			external_username = COALESCE(?, external_username),
			external_display_name = COALESCE(?, external_display_name),
			external_linked_at = CASE WHEN ? IS NOT NULL AND ? IS NOT NULL THEN ? ELSE external_linked_at END
		WHERE id = ?`,
		platform, userExtID, update.Username, update.DisplayName,
		platform, userExtID, now, userID,
	)
	if err != nil {
		return auth.User{}, fmt.Errorf("update external: %w", err)
	}
	return s.auth.GetUserByID(ctx, userID)
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func getSetting(ctx context.Context, db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_setting_kv WHERE key = ?`, key).Scan(&value)
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
