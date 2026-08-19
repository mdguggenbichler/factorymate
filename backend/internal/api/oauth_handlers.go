package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"factorymate/internal/auth"
	"factorymate/internal/registration"
)

func (h *Handler) AuthConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"discordOAuthEnabled": auth.OAuthConfigured(),
	})
}

func (h *Handler) DiscordOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !auth.OAuthConfigured() {
		writeError(w, r, http.StatusServiceUnavailable, "discord oauth is not configured")
		return
	}

	stateToken := strings.TrimSpace(r.URL.Query().Get("state"))
	if stateToken == "" {
		token, err := h.auth.CreateOAuthState(r.Context(), auth.OAuthPurposeLogin, auth.OAuthStateMeta{})
		if err != nil {
			writeError(w, r, http.StatusServiceUnavailable, "could not start oauth")
			return
		}
		stateToken = token
	}

	authorizeURL, err := h.auth.BuildOAuthAuthorizeURL(r.Context(), stateToken)
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "could not start oauth")
		return
	}
	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (h *Handler) DiscordOAuthLink(w http.ResponseWriter, r *http.Request) {
	if !auth.OAuthConfigured() {
		writeError(w, r, http.StatusServiceUnavailable, "discord oauth is not configured")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if user.External.UserID != nil && *user.External.UserID != "" {
		writeError(w, r, http.StatusConflict, "discord is already linked")
		return
	}

	token, err := h.auth.CreateOAuthState(r.Context(), auth.OAuthPurposeLink, auth.OAuthStateMeta{
		UserID: user.ID,
	})
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "could not start oauth")
		return
	}

	authorizeURL, err := h.auth.BuildOAuthAuthorizeURL(r.Context(), token)
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "could not start oauth")
		return
	}
	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

func (h *Handler) DiscordOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !auth.OAuthConfigured() {
		writeError(w, r, http.StatusServiceUnavailable, "discord oauth is not configured")
		return
	}

	stateToken := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if stateToken == "" || code == "" {
		writeError(w, r, http.StatusBadRequest, "invalid oauth callback")
		return
	}

	row, err := h.auth.ConsumeOAuthState(r.Context(), stateToken, "")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid or expired oauth state")
		return
	}

	exchange := h.auth.ExchangeDiscordOAuthCode
	if h.oauthExchange != nil {
		exchange = h.oauthExchange
	}
	discordUser, err := exchange(r.Context(), code)
	if err != nil {
		writeError(w, r, http.StatusBadGateway, "discord authorization failed")
		return
	}

	switch row.Purpose {
	case auth.OAuthPurposeLogin:
		h.oauthCallbackLogin(w, r, discordUser)
	case auth.OAuthPurposeRegister:
		h.oauthCallbackRegister(w, r, row, discordUser)
	case auth.OAuthPurposeLink:
		h.oauthCallbackLink(w, r, row, discordUser)
	case auth.OAuthPurposeRegisterComplete:
		writeError(w, r, http.StatusBadRequest, "complete registration on the web form")
	default:
		writeError(w, r, http.StatusBadRequest, "invalid oauth purpose")
	}
}

func (h *Handler) oauthCallbackLogin(w http.ResponseWriter, r *http.Request, discordUser auth.DiscordUserResponse) {
	user, err := h.auth.GetUserByExternal(r.Context(), registration.PlatformDiscord, discordUser.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		http.Redirect(w, r, frontendURL("/login?error=not_registered"), http.StatusFound)
		return
	}
	if user.Status == auth.StatusPendingApproval {
		sess, err := h.auth.CreateSession(r.Context(), user.ID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		auth.SetSessionCookie(w, r, sess.ID)
		http.Redirect(w, r, frontendURL("/awaiting-approval"), http.StatusFound)
		return
	}

	sess, err := h.auth.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	auth.SetSessionCookie(w, r, sess.ID)
	http.Redirect(w, r, frontendURL("/"), http.StatusFound)
}

func (h *Handler) oauthCallbackRegister(w http.ResponseWriter, r *http.Request, row auth.OAuthStateRow, discordUser auth.DiscordUserResponse) {
	if row.ExternalUserID != "" && row.ExternalUserID != discordUser.ID {
		writeError(w, r, http.StatusForbidden, "discord user mismatch")
		return
	}

	existing, err := h.auth.GetUserByExternal(r.Context(), registration.PlatformDiscord, discordUser.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if existing != nil {
		http.Redirect(w, r, frontendURL("/login?error=already_registered"), http.StatusFound)
		return
	}

	completeToken, err := h.auth.CreateOAuthState(r.Context(), auth.OAuthPurposeRegisterComplete, auth.OAuthStateMeta{
		ExternalUserID:      discordUser.ID,
		ExternalUsername:    discordUser.Username,
		ExternalDisplayName: auth.DiscordDisplayName(discordUser),
		ForceApprove:        row.ForceApprove,
		Role:                row.Role,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	http.Redirect(w, r, frontendURL("/register/complete?token="+url.QueryEscape(completeToken)), http.StatusFound)
}

func (h *Handler) oauthCallbackLink(w http.ResponseWriter, r *http.Request, row auth.OAuthStateRow, discordUser auth.DiscordUserResponse) {
	if row.UserID == 0 {
		writeError(w, r, http.StatusBadRequest, "invalid link state")
		return
	}

	linked, err := h.auth.LinkExternal(
		r.Context(),
		row.UserID,
		registration.PlatformDiscord,
		discordUser.ID,
		discordUser.Username,
		auth.DiscordDisplayName(discordUser),
	)
	if err != nil {
		if errors.Is(err, auth.ErrExternalAlreadyLinked) {
			http.Redirect(w, r, frontendURL("/account?error=discord_taken"), http.StatusFound)
			return
		}
		writeError(w, r, http.StatusInternalServerError, "could not link discord")
		return
	}

	sess, err := h.auth.CreateSession(r.Context(), linked.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	auth.SetSessionCookie(w, r, sess.ID)
	http.Redirect(w, r, frontendURL("/account?linked=1"), http.StatusFound)
}

type registerCompleteRequest struct {
	Token             string `json:"token"`
	Username          string `json:"username"`
	PendingPlayerName string `json:"pendingPlayerName"`
}

func (h *Handler) RegisterComplete(w http.ResponseWriter, r *http.Request) {
	if !auth.OAuthConfigured() {
		writeError(w, r, http.StatusServiceUnavailable, "discord oauth is not configured")
		return
	}

	var req registerCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.PendingPlayerName = strings.TrimSpace(req.PendingPlayerName)
	if req.Token == "" || req.Username == "" || req.PendingPlayerName == "" {
		writeError(w, r, http.StatusBadRequest, "token, username, and pendingPlayerName are required")
		return
	}

	row, err := h.auth.ConsumeOAuthState(r.Context(), req.Token, auth.OAuthPurposeRegisterComplete)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid or expired registration token")
		return
	}
	if row.ExternalUserID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid registration token")
		return
	}

	ext := registration.ExternalIdentity{
		Platform:    registration.PlatformDiscord,
		UserID:      row.ExternalUserID,
		Username:    row.ExternalUsername,
		DisplayName: row.ExternalDisplayName,
	}

	result, err := h.registration.Register(r.Context(), registration.RegisterParams{
		Username:          req.Username,
		Password:          "",
		PendingPlayerName: req.PendingPlayerName,
		External:          ext,
		Role:              row.Role,
		ForceApprove:      row.ForceApprove,
	})
	if err != nil {
		if errors.Is(err, registration.ErrAlreadyRegistered) {
			writeError(w, r, http.StatusConflict, "discord account already registered")
			return
		}
		writeError(w, r, http.StatusBadRequest, "registration failed")
		return
	}

	if result.PendingApproval {
		sess, err := h.auth.CreateSession(r.Context(), result.User.ID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		auth.SetSessionCookie(w, r, sess.ID)
		writeJSON(w, http.StatusCreated, map[string]any{
			"user":            result.User,
			"pendingApproval": true,
		})
		return
	}

	sess, err := h.auth.CreateSession(r.Context(), result.User.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	auth.SetSessionCookie(w, r, sess.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"user":            result.User,
		"pendingApproval": false,
	})
}

func frontendURL(path string) string {
	base := auth.PublicBaseURL()
	if base == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}
