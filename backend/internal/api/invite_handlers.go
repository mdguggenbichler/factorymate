package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"factorymate/internal/auth"

	"github.com/go-chi/chi/v5"
)

type createInviteRequest struct {
	Role auth.Role `json:"role"`
}

func inviteToJSON(inv auth.Invite) map[string]any {
	out := map[string]any{
		"id":        inv.ID,
		"token":     inv.Token,
		"role":      inv.Role,
		"createdBy": inv.CreatedBy,
		"createdAt": inv.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"expiresAt": inv.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
		"status":    inv.Status,
		"invitePath": "/invite/" + inv.Token,
	}
	if inv.AcceptedAt != nil {
		out["acceptedAt"] = inv.AcceptedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if inv.AcceptedByUserID != nil {
		out["acceptedByUserId"] = *inv.AcceptedByUserID
	}
	if inv.AcceptedUsername != nil {
		out["acceptedUsername"] = *inv.AcceptedUsername
	}
	if inv.RevokedAt != nil {
		out["revokedAt"] = inv.RevokedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return out
}

func (h *Handler) GetInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeError(w, r, http.StatusBadRequest, "token is required")
		return
	}

	inv, err := h.auth.GetInviteByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, auth.ErrInviteNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	if inv.Status != auth.InviteStatusPending {
		writeError(w, r, http.StatusGone, string(inv.Status))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"role":      inv.Role,
		"expiresAt": inv.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
		"status":    inv.Status,
	})
}

type acceptInviteRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeError(w, r, http.StatusBadRequest, "token is required")
		return
	}

	var req acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.auth.AcceptInvite(r.Context(), token, req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInviteNotFound):
			writeError(w, r, http.StatusNotFound, "not found")
		case errors.Is(err, auth.ErrInviteExpired):
			writeError(w, r, http.StatusGone, "expired")
		case errors.Is(err, auth.ErrInviteNotPending):
			writeError(w, r, http.StatusGone, "not pending")
		case errors.Is(err, auth.ErrDuplicateUsername):
			writeError(w, r, http.StatusConflict, "username already exists")
		case errors.Is(err, auth.ErrWeakPassword):
			writeError(w, r, http.StatusBadRequest, err.Error())
		default:
			if strings.Contains(err.Error(), "required") {
				writeError(w, r, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal error")
		}
		return
	}

	sess, err := h.auth.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	auth.SetSessionCookie(w, r, sess.ID)
	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role != auth.RoleAdmin && req.Role != auth.RoleViewer {
		writeError(w, r, http.StatusBadRequest, "invalid role")
		return
	}

	inv, err := h.auth.CreateInvite(r.Context(), user.ID, req.Role, auth.DefaultInviteTTL)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, inviteToJSON(inv))
}

func (h *Handler) ListInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := h.auth.ListInvites(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]map[string]any, 0, len(invites))
	for _, inv := range invites {
		out = append(out, inviteToJSON(inv))
	}
	writeJSON(w, http.StatusOK, map[string]any{"invites": out})
}

func (h *Handler) RevokeInvite(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.auth.RevokeInvite(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, auth.ErrInviteNotFound):
			writeError(w, r, http.StatusNotFound, "not found")
		case errors.Is(err, auth.ErrInviteNotPending):
			writeError(w, r, http.StatusBadRequest, "invite is not pending")
		default:
			writeError(w, r, http.StatusInternalServerError, "internal error")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
