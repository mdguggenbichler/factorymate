package api

import (
	"encoding/json"
	"net/http"

	"factorymate/internal/auth"
	"factorymate/internal/notifications"
)

func (h *Handler) GetAccountNotifications(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}

	prefs, err := h.notifications.GetUserPrefs(r.Context(), user.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (h *Handler) PutAccountNotifications(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var input notifications.UserPrefs
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Categories == nil {
		input.Categories = map[string]bool{}
	}

	prefs, err := h.notifications.SetUserPrefs(r.Context(), user.ID, input)
	if err != nil {
		if err.Error() == "user not found" {
			writeError(w, r, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (h *Handler) GetNotificationDefaults(w http.ResponseWriter, r *http.Request) {
	defaults, err := h.notifications.GetAdminDefaults(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, defaults)
}

func (h *Handler) PutNotificationDefaults(w http.ResponseWriter, r *http.Request) {
	var input notifications.AdminDefaults
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Categories == nil {
		input.Categories = map[string]bool{}
	}

	defaults, err := h.notifications.SetAdminDefaults(r.Context(), input)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, defaults)
}
