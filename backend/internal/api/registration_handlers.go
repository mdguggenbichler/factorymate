package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"factorymate/internal/auth"
	"factorymate/internal/registration"

	"github.com/go-chi/chi/v5"
)

type rejectRegistrationRequest struct {
	Comment string `json:"comment"`
}

type updateExternalRequest struct {
	ExternalPlatform    *string `json:"externalPlatform"`
	ExternalUserID      *string `json:"externalUserId"`
	ExternalUsername    *string `json:"externalUsername"`
	ExternalDisplayName *string `json:"externalDisplayName"`
	Unlink              bool    `json:"unlink"`
}

func (h *Handler) ListPendingRegistrations(w http.ResponseWriter, r *http.Request) {
	pending, err := h.registration.ListPending(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if pending == nil {
		pending = []registration.PendingRegistration{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"registrations": pending})
}

func (h *Handler) ApproveRegistration(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	admin, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}

	user, err := h.registration.ApproveRegistration(r.Context(), id, admin.ID)
	if err != nil {
		if errors.Is(err, registration.ErrNotPendingApproval) {
			writeError(w, r, http.StatusConflict, "registration is not pending")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	if h.discord != nil && user.External.UserID != nil {
		h.discord.SendWelcomeDM(r.Context(), *user.External.UserID, user.Username)
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) RejectRegistration(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	var req rejectRegistrationRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	admin, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}

	externalUserID, err := h.registration.RejectRegistration(r.Context(), id, admin.ID, req.Comment)
	if err != nil {
		if errors.Is(err, registration.ErrNotPendingApproval) {
			writeError(w, r, http.StatusConflict, "registration is not pending")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	if h.discord != nil && externalUserID != "" {
		h.discord.SendRegistrationDeclinedDM(r.Context(), externalUserID, req.Comment)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListUnmappedPlayers(w http.ResponseWriter, r *http.Request) {
	players, err := h.registration.ListUnmappedPlayers(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if players == nil {
		players = []registration.UnmappedPlayer{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"players": players})
}

func (h *Handler) UpdateUserExternal(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	var req updateExternalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.registration.UpdateExternalIdentity(r.Context(), id, registration.ExternalUpdate{
		Platform:    req.ExternalPlatform,
		UserID:      req.ExternalUserID,
		Username:    req.ExternalUsername,
		DisplayName: req.ExternalDisplayName,
		Unlink:      req.Unlink,
	})
	if err != nil {
		if errors.Is(err, registration.ErrUserNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		if errors.Is(err, registration.ErrAlreadyRegistered) {
			writeError(w, r, http.StatusConflict, "external identity already in use")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, user)
}
