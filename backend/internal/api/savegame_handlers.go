package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"factorymate/internal/auth"
	"factorymate/internal/savegame"
)

func (h *Handler) GetSavegame(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	result, err := h.savegame.DownloadLatest(r.Context(), user.ID, savegame.ChannelWeb)
	if err != nil {
		writeSavegameError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+result.Filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Body)
}

func (h *Handler) GetSavegameStatus(w http.ResponseWriter, r *http.Request) {
	configured, err := h.savegame.IsConfigured(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	resp := map[string]any{"configured": configured}
	if configured {
		info, err := h.savegame.ResolveLatest(r.Context())
		if err == nil {
			resp["activeSessionName"] = info.ActiveSessionName
			resp["latestSaveName"] = info.SaveName
			resp["saveDateTime"] = info.SaveDateTime
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type gameAPITestRequest struct {
	GameAPIHost  string `json:"gameApiHost"`
	GameAPIPort  int    `json:"gameApiPort"`
	GameAPIToken string `json:"gameApiToken"`
}

func (h *Handler) TestGameAPIConnection(w http.ResponseWriter, r *http.Request) {
	var req gameAPITestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.GameAPIPort <= 0 {
		req.GameAPIPort = 7777
	}
	if req.GameAPIHost == "" {
		writeError(w, r, http.StatusBadRequest, "gameApiHost is required")
		return
	}

	cfg, err := h.resolveGameAPITestConfig(r, req)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "game API token is required")
		return
	}

	info, err := h.savegame.TestConnection(r.Context(), cfg)
	if err != nil {
		writeError(w, r, http.StatusBadGateway, "could not reach dedicated server API")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"reachable":         true,
		"activeSessionName": info.ActiveSessionName,
		"latestSaveName":    info.SaveName,
	})
}

func (h *Handler) resolveGameAPITestConfig(r *http.Request, req gameAPITestRequest) (savegame.Config, error) {
	token := req.GameAPIToken
	if token == "" {
		stored, err := h.savegame.ConfigFromDB(r.Context())
		if err != nil {
			return savegame.Config{}, err
		}
		token = stored.Token
	}
	if token == "" {
		return savegame.Config{}, savegame.ErrNotConfigured
	}
	return savegame.Config{
		Host:  req.GameAPIHost,
		Port:  req.GameAPIPort,
		Token: token,
	}, nil
}

func writeSavegameError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, savegame.ErrNotConfigured):
		writeError(w, r, http.StatusServiceUnavailable, "save download is not configured")
	case errors.Is(err, savegame.ErrRateLimited):
		writeError(w, r, http.StatusTooManyRequests, "please wait before downloading again")
	case errors.Is(err, savegame.ErrNoActiveSave):
		writeError(w, r, http.StatusNotFound, "no save available")
	default:
		writeError(w, r, http.StatusBadGateway, "could not download save")
	}
}
