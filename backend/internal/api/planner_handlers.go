package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) GetPlannerCatalog(w http.ResponseWriter, r *http.Request) {
	if h.planner == nil {
		writeError(w, r, http.StatusInternalServerError, "planner catalog unavailable")
		return
	}
	writeJSON(w, http.StatusOK, h.planner)
}

func (h *Handler) GetPlannerIcon(w http.ResponseWriter, r *http.Request) {
	if h.planner == nil {
		writeError(w, r, http.StatusInternalServerError, "planner catalog unavailable")
		return
	}

	className := chi.URLParam(r, "className")
	if className == "" {
		writeError(w, r, http.StatusBadRequest, "className required")
		return
	}

	iconClass := h.planner.ResolveIconClassName(className)
	path := filepath.Join(h.plannerIcons, iconClass+".png")
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, r, path)
}
