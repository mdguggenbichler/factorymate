package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"factorymate/internal/auth"
)

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	count, err := h.auth.UserCount(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if count > 0 {
		writeError(w, r, http.StatusForbidden, "setup already completed")
		return
	}

	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeError(w, r, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := h.auth.CreateUser(r.Context(), req.Username, req.Password, auth.RoleAdmin)
	if err != nil {
		if errors.Is(err, auth.ErrWeakPassword) {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
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

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeError(w, r, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := h.auth.CheckCredentials(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, r, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	if user.Status == auth.StatusPendingApproval {
		sess, err := h.auth.CreateSession(r.Context(), user.ID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		auth.SetSessionCookie(w, r, sess.ID)
		writeError(w, r, http.StatusForbidden, "account pending approval")
		return
	}

	sess, err := h.auth.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	auth.SetSessionCookie(w, r, sess.ID)
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.CookieName); err == nil && cookie.Value != "" {
		_ = h.auth.DeleteSession(r.Context(), cookie.Value)
	}
	auth.ClearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	me, err := h.auth.GetMeUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, me)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		writeError(w, r, http.StatusBadRequest, "password is required")
		return
	}

	if err := h.auth.UpdatePassword(r.Context(), user.ID, req.Password); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		if errors.Is(err, auth.ErrWeakPassword) {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
