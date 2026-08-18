package api

import (
	"encoding/json"
	"net/http"

	"factorymate/internal/auth"
	"factorymate/internal/connection"
)

func (h *Handler) GetConnectionDetails(w http.ResponseWriter, r *http.Request) {
	details, err := h.connection.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, details)
}

func (h *Handler) PutConnectionDetails(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var input connection.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	details, err := h.connection.Set(r.Context(), input, user.ID)
	if err != nil {
		if err.Error() == "gameHost is required" || err.Error() == "gamePort must be positive" {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, details)
}
