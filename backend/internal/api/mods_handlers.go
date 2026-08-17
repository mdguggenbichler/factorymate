package api

import (
	"net/http"

	"factorymate/internal/mods"
)

func (h *Handler) GetMods(w http.ResponseWriter, r *http.Request) {
	list, err := h.mods.List(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if list.Mods == nil {
		list.Mods = []mods.Mod{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) GetModsSMMProfile(w http.ResponseWriter, r *http.Request) {
	data, filename, err := h.mods.GenerateSMMProfile(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "could not generate SMM profile")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) PostModsRefresh(w http.ResponseWriter, r *http.Request) {
	list, err := h.mods.Refresh(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if list.Mods == nil {
		list.Mods = []mods.Mod{}
	}
	writeJSON(w, http.StatusOK, list)
}
