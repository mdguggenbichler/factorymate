package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const oauthStateTTL = 10 * time.Minute

// OAuthPurpose identifies why an OAuth flow was started.
type OAuthPurpose string

const (
	OAuthPurposeLogin            OAuthPurpose = "login"
	OAuthPurposeRegister         OAuthPurpose = "register"
	OAuthPurposeLink             OAuthPurpose = "link"
	OAuthPurposeRegisterComplete OAuthPurpose = "register_complete"
)

// OAuthStateMeta holds optional fields stored with an OAuth state row.
type OAuthStateMeta struct {
	ExternalUserID      string
	ExternalUsername    string
	ExternalDisplayName string
	ForceApprove        bool
	Role                Role
	UserID              int64
}

type OAuthStateRow struct {
	Purpose             OAuthPurpose
	ExternalUserID      string
	ExternalUsername    string
	ExternalDisplayName string
	ForceApprove        bool
	Role                Role
	UserID              int64
}

type DiscordUserResponse struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name"`
	Discriminator string `json:"discriminator"`
}

type discordTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// OAuthConfigured reports whether Discord SSO is available (client secret + public URL).
func OAuthConfigured() bool {
	return strings.TrimSpace(os.Getenv("DISCORD_CLIENT_SECRET")) != "" &&
		strings.TrimSpace(PublicBaseURL()) != ""
}

func PublicBaseURL() string {
	v := strings.TrimSpace(os.Getenv("FACTORYMATE_PUBLIC_URL"))
	return strings.TrimRight(v, "/")
}

func oauthRedirectURI() string {
	return PublicBaseURL() + "/api/auth/discord/callback"
}

// ApplicationClientID resolves the Discord application ID from the bot token via REST.
func ApplicationClientID(ctx context.Context) (string, error) {
	token := strings.TrimSpace(os.Getenv("DISCORD_BOT_TOKEN"))
	if token == "" {
		return "", fmt.Errorf("DISCORD_BOT_TOKEN is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/oauth2/applications/@me", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("discord application request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("discord application status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var app struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return "", fmt.Errorf("decode discord application: %w", err)
	}
	if strings.TrimSpace(app.ID) == "" {
		return "", fmt.Errorf("discord application id missing")
	}
	return app.ID, nil
}

// CreateOAuthState stores a hashed nonce and returns the raw token for URLs.
func (s *Service) CreateOAuthState(ctx context.Context, purpose OAuthPurpose, meta OAuthStateMeta) (string, error) {
	if !OAuthConfigured() {
		return "", ErrOAuthNotConfigured
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate oauth nonce: %w", err)
	}
	token := hex.EncodeToString(raw)
	hash := hashOAuthToken(token)

	now := time.Now().UTC()
	expires := now.Add(oauthStateTTL)
	role := ""
	if meta.Role != "" {
		role = string(meta.Role)
	}

	force := 0
	if meta.ForceApprove {
		force = 1
	}

	var userID any
	if meta.UserID > 0 {
		userID = meta.UserID
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_states (
			token_hash, purpose, external_user_id, external_username, external_display_name,
			force_approve, fm_role, user_id, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		hash, string(purpose),
		nullIfEmpty(meta.ExternalUserID),
		nullIfEmpty(meta.ExternalUsername),
		nullIfEmpty(meta.ExternalDisplayName),
		force, nullIfEmpty(role), userID,
		now.Format(time.RFC3339), expires.Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("insert oauth state: %w", err)
	}
	return token, nil
}

// ConsumeOAuthState validates and marks a state token single-use.
func (s *Service) ConsumeOAuthState(ctx context.Context, token string, expectedPurpose OAuthPurpose) (OAuthStateRow, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return OAuthStateRow{}, ErrInvalidOAuthState
	}
	hash := hashOAuthToken(token)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OAuthStateRow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var row OAuthStateRow
	var purpose, extUserID, extUsername, extDisplay, fmRole sql.NullString
	var forceApprove int
	var userID sql.NullInt64
	var expiresAt, usedAt sql.NullString

	err = tx.QueryRowContext(ctx, `
		SELECT purpose, external_user_id, external_username, external_display_name,
			force_approve, fm_role, user_id, expires_at, used_at
		FROM oauth_states WHERE token_hash = ?`, hash,
	).Scan(&purpose, &extUserID, &extUsername, &extDisplay, &forceApprove, &fmRole, &userID, &expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return OAuthStateRow{}, ErrInvalidOAuthState
	}
	if err != nil {
		return OAuthStateRow{}, fmt.Errorf("query oauth state: %w", err)
	}
	if usedAt.Valid && usedAt.String != "" {
		return OAuthStateRow{}, ErrInvalidOAuthState
	}
	if expectedPurpose != "" && purpose.String != string(expectedPurpose) {
		return OAuthStateRow{}, ErrInvalidOAuthState
	}

	exp, err := time.Parse(time.RFC3339, expiresAt.String)
	if err != nil || time.Now().UTC().After(exp) {
		return OAuthStateRow{}, ErrInvalidOAuthState
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := tx.ExecContext(ctx, `
		UPDATE oauth_states SET used_at = ? WHERE token_hash = ? AND used_at IS NULL`, now, hash)
	if err != nil {
		return OAuthStateRow{}, fmt.Errorf("mark oauth state used: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return OAuthStateRow{}, ErrInvalidOAuthState
	}

	row.Purpose = OAuthPurpose(purpose.String)
	if extUserID.Valid {
		row.ExternalUserID = extUserID.String
	}
	if extUsername.Valid {
		row.ExternalUsername = extUsername.String
	}
	if extDisplay.Valid {
		row.ExternalDisplayName = extDisplay.String
	}
	row.ForceApprove = forceApprove != 0
	if fmRole.Valid && fmRole.String != "" {
		row.Role = Role(fmRole.String)
	}
	if userID.Valid {
		row.UserID = userID.Int64
	}

	if err := tx.Commit(); err != nil {
		return OAuthStateRow{}, fmt.Errorf("commit oauth state: %w", err)
	}
	return row, nil
}

// BuildOAuthAuthorizeURL returns the Discord authorize URL for an existing state token.
func (s *Service) BuildOAuthAuthorizeURL(ctx context.Context, stateToken string) (string, error) {
	if !OAuthConfigured() {
		return "", ErrOAuthNotConfigured
	}
	clientID, err := ApplicationClientID(ctx)
	if err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", oauthRedirectURI())
	q.Set("response_type", "code")
	q.Set("scope", "identify")
	q.Set("state", stateToken)

	return "https://discord.com/api/oauth2/authorize?" + q.Encode(), nil
}

// ExchangeDiscordOAuthCode trades an authorization code for the Discord user profile.
func (s *Service) ExchangeDiscordOAuthCode(ctx context.Context, code string) (DiscordUserResponse, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return DiscordUserResponse{}, fmt.Errorf("authorization code required")
	}
	clientSecret := strings.TrimSpace(os.Getenv("DISCORD_CLIENT_SECRET"))
	if clientSecret == "" {
		return DiscordUserResponse{}, ErrOAuthNotConfigured
	}
	clientID, err := ApplicationClientID(ctx)
	if err != nil {
		return DiscordUserResponse{}, err
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", oauthRedirectURI())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://discord.com/api/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return DiscordUserResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DiscordUserResponse{}, fmt.Errorf("discord token exchange: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return DiscordUserResponse{}, fmt.Errorf("discord token status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token discordTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return DiscordUserResponse{}, fmt.Errorf("decode discord token: %w", err)
	}

	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/users/@me", nil)
	if err != nil {
		return DiscordUserResponse{}, err
	}
	userReq.Header.Set("Authorization", token.TokenType+" "+token.AccessToken)

	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		return DiscordUserResponse{}, fmt.Errorf("discord user request: %w", err)
	}
	defer userResp.Body.Close()
	if userResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(userResp.Body, 512))
		return DiscordUserResponse{}, fmt.Errorf("discord user status %d: %s", userResp.StatusCode, strings.TrimSpace(string(body)))
	}

	var user DiscordUserResponse
	if err := json.NewDecoder(userResp.Body).Decode(&user); err != nil {
		return DiscordUserResponse{}, fmt.Errorf("decode discord user: %w", err)
	}
	return user, nil
}

// GetUserByExternal returns an active or pending user linked to a Discord ID.
func (s *Service) GetUserByExternal(ctx context.Context, platform, externalUserID string) (*User, error) {
	platform = strings.TrimSpace(platform)
	externalUserID = strings.TrimSpace(externalUserID)
	if platform == "" || externalUserID == "" {
		return nil, nil
	}

	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM users WHERE external_platform = ? AND external_user_id = ?`,
		platform, externalUserID,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query external user: %w", err)
	}
	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// LinkExternal attaches a Discord identity to an existing user (logged-in link flow).
func (s *Service) LinkExternal(ctx context.Context, userID int64, platform, externalUserID, username, displayName string) (User, error) {
	platform = strings.TrimSpace(platform)
	externalUserID = strings.TrimSpace(externalUserID)
	if platform == "" || externalUserID == "" {
		return User{}, fmt.Errorf("external identity required")
	}

	existing, err := s.GetUserByExternal(ctx, platform, externalUserID)
	if err != nil {
		return User{}, err
	}
	if existing != nil && existing.ID != userID {
		return User{}, ErrExternalAlreadyLinked
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
		platform, externalUserID, nullIfEmpty(username), nullIfEmpty(displayName), now, userID,
	)
	if err != nil {
		return User{}, fmt.Errorf("link external: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var currentExt sql.NullString
		err := s.db.QueryRowContext(ctx, `SELECT external_user_id FROM users WHERE id = ?`, userID).Scan(&currentExt)
		if err == sql.ErrNoRows {
			return User{}, ErrUserNotFound
		}
		if err != nil {
			return User{}, fmt.Errorf("query user external: %w", err)
		}
		if currentExt.Valid && currentExt.String != "" {
			return User{}, ErrExternalAlreadyLinked
		}
		return User{}, ErrUserNotFound
	}
	return s.GetUserByID(ctx, userID)
}

// CleanupExpiredOAuthStates removes expired oauth_states rows.
func (s *Service) CleanupExpiredOAuthStates(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `DELETE FROM oauth_states WHERE expires_at < ?`, now)
	return err
}

func hashOAuthToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func DiscordDisplayName(user DiscordUserResponse) string {
	if strings.TrimSpace(user.GlobalName) != "" {
		return user.GlobalName
	}
	if user.Discriminator != "" && user.Discriminator != "0" {
		return user.Username + "#" + user.Discriminator
	}
	return user.Username
}
