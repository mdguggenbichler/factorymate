package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"factorymate/internal/auth"
	"factorymate/internal/planner"

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

func (h *Handler) ListPlannerPlans(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}

	includeArchived := strings.EqualFold(r.URL.Query().Get("includeArchived"), "true")
	statusFilters := parsePlanStatusFilters(r.URL.Query()["status"])
	if raw := r.URL.Query().Get("status"); raw != "" && len(statusFilters) == 0 {
		writeError(w, r, http.StatusBadRequest, "invalid status filter")
		return
	}

	plans, err := h.listFactoryPlans(r.Context(), user, includeArchived, statusFilters)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
}

func (h *Handler) CreatePlannerPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req struct {
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, r, http.StatusBadRequest, "name is required")
		return
	}

	visibility := "private"
	if req.Visibility != "" {
		if req.Visibility != "private" && req.Visibility != "shared" {
			writeError(w, r, http.StatusBadRequest, "invalid visibility")
			return
		}
		visibility = req.Visibility
	}

	statusDB := "planning"
	if req.Status != "" {
		s, err := planStatusFromAPI(req.Status)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		statusDB = s
	}

	now := timeNowRFC3339()
	emptyGraph, _ := json.Marshal(planner.EmptyPlanGraph())
	res, err := h.db.ExecContext(r.Context(), `
		INSERT INTO factory_plans (
			owner_user_id, name, visibility, status, solver_options_json, graph_json,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, '{}', ?, ?, ?)`,
		user.ID, req.Name, visibility, statusDB, string(emptyGraph), now, now,
	)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	id, _ := res.LastInsertId()
	plan, err := h.getFactoryPlan(r.Context(), id, user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (h *Handler) GetPlannerPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	plan, err := h.getFactoryPlan(r.Context(), planID, user)
	if err != nil {
		if errors.Is(err, planner.ErrPlanNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		if errors.Is(err, errPlanForbidden) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) PatchPlannerPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	row, err := h.loadPlanRow(r.Context(), planID)
	if err != nil {
		if errors.Is(err, planner.ErrPlanNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManagePlan(user, row) {
		writeError(w, r, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		Name       *string `json:"name"`
		Visibility *string `json:"visibility"`
		Status     *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	name := row.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, r, http.StatusBadRequest, "name is required")
			return
		}
	}
	visibility := row.Visibility
	if req.Visibility != nil {
		if *req.Visibility != "private" && *req.Visibility != "shared" {
			writeError(w, r, http.StatusBadRequest, "invalid visibility")
			return
		}
		visibility = *req.Visibility
	}
	status := row.Status
	if req.Status != nil {
		s, err := planStatusFromAPI(*req.Status)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		status = s
	}

	updatedAt := timeNowRFC3339()
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE factory_plans
		SET name = ?, visibility = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		name, visibility, status, updatedAt, planID,
	); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if status == "archived" {
		if err := planner.ClearPlanLock(r.Context(), tx, planID); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	plan, err := h.getFactoryPlan(r.Context(), planID, user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) DeletePlannerPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	row, err := h.loadPlanRow(r.Context(), planID)
	if err != nil {
		if errors.Is(err, planner.ErrPlanNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if !canManagePlan(user, row) {
		writeError(w, r, http.StatusForbidden, "forbidden")
		return
	}

	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM factory_plans WHERE id = ?`, planID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PutPlannerPlanGraph(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	row, err := h.loadPlanRow(r.Context(), planID)
	if err != nil {
		if errors.Is(err, planner.ErrPlanNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if !canReadPlan(user, row) {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if row.Status == "archived" {
		writeError(w, r, http.StatusForbidden, planner.ErrPlanArchived.Error())
		return
	}
	if err := planner.RequireLockHolder(r.Context(), h.db, planID, user.ID); err != nil {
		switch {
		case errors.Is(err, planner.ErrLockRequired):
			writeError(w, r, http.StatusConflict, err.Error())
		default:
			writeError(w, r, http.StatusConflict, err.Error())
		}
		return
	}

	var req struct {
		Graph     planner.PlanGraph `json:"graph"`
		UpdatedAt string            `json:"updatedAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UpdatedAt == "" {
		writeError(w, r, http.StatusBadRequest, "updatedAt is required")
		return
	}
	if req.UpdatedAt != row.UpdatedAt {
		writeError(w, r, http.StatusConflict, planner.ErrUpdatedAtMismatch.Error())
		return
	}

	graphJSON, err := json.Marshal(req.Graph)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid graph")
		return
	}
	updatedAt := timeNowRFC3339()
	res, err := h.db.ExecContext(r.Context(), `
		UPDATE factory_plans SET graph_json = ?, updated_at = ?
		WHERE id = ? AND updated_at = ?`,
		string(graphJSON), updatedAt, planID, row.UpdatedAt,
	)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, r, http.StatusConflict, planner.ErrUpdatedAtMismatch.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"updatedAt": updatedAt})
}

func (h *Handler) PostPlannerSuggest(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	row, err := h.loadPlanRow(r.Context(), planID)
	if err != nil {
		if errors.Is(err, planner.ErrPlanNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if !canReadPlan(user, row) {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if row.Status == "archived" {
		writeError(w, r, http.StatusForbidden, planner.ErrPlanArchived.Error())
		return
	}
	if err := planner.RequireLockHolder(r.Context(), h.db, planID, user.ID); err != nil {
		writeError(w, r, http.StatusConflict, err.Error())
		return
	}

	var req struct {
		ItemClass              string            `json:"itemClass"`
		RatePerMin             float64           `json:"ratePerMin"`
		RecipeByProductClass   map[string]string `json:"recipeByProductClass"`
		Apply                  bool              `json:"apply"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	graph, err := h.runSuggest(req.ItemClass, req.RatePerMin, req.RecipeByProductClass)
	if err != nil {
		var cycle *planner.CycleError
		if errors.As(err, &cycle) {
			writeError(w, r, http.StatusBadRequest, cycle.Error())
			return
		}
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	if req.Apply {
		if err := h.persistSuggestGraph(r.Context(), planID, row.UpdatedAt, req.ItemClass, req.RatePerMin, graph); err != nil {
			if errors.Is(err, planner.ErrUpdatedAtMismatch) {
				writeError(w, r, http.StatusConflict, err.Error())
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"graph": graph})
}

func (h *Handler) PostPlannerApplySuggest(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	row, err := h.loadPlanRow(r.Context(), planID)
	if err != nil {
		if errors.Is(err, planner.ErrPlanNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if !canReadPlan(user, row) {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if row.Status == "archived" {
		writeError(w, r, http.StatusForbidden, planner.ErrPlanArchived.Error())
		return
	}
	if err := planner.RequireLockHolder(r.Context(), h.db, planID, user.ID); err != nil {
		writeError(w, r, http.StatusConflict, err.Error())
		return
	}

	var req struct {
		Graph           planner.PlanGraph `json:"graph"`
		ItemClass       string            `json:"itemClass"`
		RatePerMin      float64           `json:"ratePerMin"`
		UpdatedAt       string            `json:"updatedAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UpdatedAt == "" {
		req.UpdatedAt = row.UpdatedAt
	}
	if err := h.persistSuggestGraph(r.Context(), planID, req.UpdatedAt, req.ItemClass, req.RatePerMin, req.Graph); err != nil {
		if errors.Is(err, planner.ErrUpdatedAtMismatch) {
			writeError(w, r, http.StatusConflict, err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	plan, err := h.getFactoryPlan(r.Context(), planID, user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) PostPlannerResetBaseline(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	row, err := h.loadPlanRow(r.Context(), planID)
	if err != nil {
		if errors.Is(err, planner.ErrPlanNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if !canReadPlan(user, row) {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if row.Status == "archived" {
		writeError(w, r, http.StatusForbidden, planner.ErrPlanArchived.Error())
		return
	}
	if err := planner.RequireLockHolder(r.Context(), h.db, planID, user.ID); err != nil {
		writeError(w, r, http.StatusConflict, err.Error())
		return
	}
	if !row.BaselineJSON.Valid || row.BaselineJSON.String == "" {
		writeError(w, r, http.StatusBadRequest, planner.ErrNoBaseline.Error())
		return
	}

	updatedAt := timeNowRFC3339()
	if _, err := h.db.ExecContext(r.Context(), `
		UPDATE factory_plans SET graph_json = ?, updated_at = ? WHERE id = ?`,
		row.BaselineJSON.String, updatedAt, planID,
	); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	plan, err := h.getFactoryPlan(r.Context(), planID, user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) PostPlannerLock(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	row, err := h.loadPlanRow(r.Context(), planID)
	if err != nil {
		if errors.Is(err, planner.ErrPlanNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if !canReadPlan(user, row) {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}

	if err := planner.AcquireLock(r.Context(), h.db, planID, user.ID); err != nil {
		switch {
		case errors.Is(err, planner.ErrPlanArchived):
			writeError(w, r, http.StatusForbidden, err.Error())
		case errors.Is(err, planner.ErrLockHeld):
			writeError(w, r, http.StatusConflict, err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal error")
		}
		return
	}
	lock, _ := planner.LoadLockState(r.Context(), h.db, planID, user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"lock": lock})
}

func (h *Handler) PostPlannerLockHeartbeat(w http.ResponseWriter, r *http.Request) {
	h.plannerLockAction(w, r, planner.HeartbeatLock)
}

func (h *Handler) PostPlannerLockRelease(w http.ResponseWriter, r *http.Request) {
	h.plannerLockAction(w, r, planner.ReleaseLock)
}

func (h *Handler) PostPlannerLockForceRelease(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	row, err := h.loadPlanRow(r.Context(), planID)
	if err != nil {
		if errors.Is(err, planner.ErrPlanNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if user.Role != auth.RoleAdmin && user.ID != row.OwnerUserID {
		writeError(w, r, http.StatusForbidden, "forbidden")
		return
	}

	if err := planner.ForceReleaseLock(r.Context(), h.db, planID, user.ID, user.Role == auth.RoleAdmin); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	lock, _ := planner.LoadLockState(r.Context(), h.db, planID, user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"lock": lock})
}

func (h *Handler) PostPlannerAnalyze(w http.ResponseWriter, r *http.Request) {
	if h.planner == nil {
		writeError(w, r, http.StatusInternalServerError, "planner catalog unavailable")
		return
	}
	var req struct {
		Graph planner.PlanGraph `json:"graph"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	result := h.planner.AnalyzeGraph(planner.ToBalanceGraph(req.Graph))
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) plannerLockAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, *sql.DB, int64, int64) error) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated")
		return
	}
	planID, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid plan id")
		return
	}

	if err := fn(r.Context(), h.db, planID, user.ID); err != nil {
		switch {
		case errors.Is(err, planner.ErrPlanNotFound):
			writeError(w, r, http.StatusNotFound, "not found")
		case errors.Is(err, planner.ErrPlanArchived):
			writeError(w, r, http.StatusForbidden, err.Error())
		case errors.Is(err, planner.ErrLockRequired):
			writeError(w, r, http.StatusConflict, err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal error")
		}
		return
	}
	lock, _ := planner.LoadLockState(r.Context(), h.db, planID, user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"lock": lock})
}

func (h *Handler) runSuggest(itemClass string, rate float64, recipeBy map[string]string) (planner.PlanGraph, error) {
	if h.planner == nil {
		return planner.PlanGraph{}, errors.New("planner catalog unavailable")
	}
	return planner.Suggest(h.planner, planner.SuggestRequest{
		ItemClass:            itemClass,
		RatePerMin:           rate,
		RecipeByProductClass: recipeBy,
		DefaultClockPercent:  100,
	})
}

func (h *Handler) persistSuggestGraph(ctx context.Context, planID int64, updatedAt, itemClass string, rate float64, graph planner.PlanGraph) error {
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		return err
	}
	newUpdatedAt := timeNowRFC3339()
	res, err := h.db.ExecContext(ctx, `
		UPDATE factory_plans
		SET graph_json = ?, baseline_json = ?, target_item_class = ?, target_rate = ?, updated_at = ?
		WHERE id = ? AND updated_at = ?`,
		string(graphJSON), string(graphJSON), nullString(itemClass), rate, newUpdatedAt, planID, updatedAt,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return planner.ErrUpdatedAtMismatch
	}
	return nil
}

type factoryPlanRow struct {
	ID                 int64
	OwnerUserID        int64
	OwnerUsername      string
	Name               string
	Visibility         string
	Status             string
	TargetItemClass    sql.NullString
	TargetRate         sql.NullFloat64
	SolverOptionsJSON  string
	GraphJSON          string
	BaselineJSON       sql.NullString
	UpdatedAt          string
}

var errPlanForbidden = errors.New("forbidden")

func (h *Handler) loadPlanRow(ctx context.Context, planID int64) (factoryPlanRow, error) {
	var row factoryPlanRow
	err := h.db.QueryRowContext(ctx, `
		SELECT fp.id, fp.owner_user_id, u.username, fp.name, fp.visibility, fp.status,
			fp.target_item_class, fp.target_rate, fp.solver_options_json, fp.graph_json,
			fp.baseline_json, fp.updated_at
		FROM factory_plans fp
		JOIN users u ON u.id = fp.owner_user_id
		WHERE fp.id = ?`,
		planID,
	).Scan(
		&row.ID, &row.OwnerUserID, &row.OwnerUsername, &row.Name, &row.Visibility, &row.Status,
		&row.TargetItemClass, &row.TargetRate, &row.SolverOptionsJSON, &row.GraphJSON,
		&row.BaselineJSON, &row.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return row, planner.ErrPlanNotFound
	}
	return row, err
}

func canReadPlan(user auth.User, row factoryPlanRow) bool {
	if user.Role == auth.RoleAdmin {
		return true
	}
	if row.Visibility == "shared" {
		return true
	}
	return row.OwnerUserID == user.ID
}

func canManagePlan(user auth.User, row factoryPlanRow) bool {
	return user.Role == auth.RoleAdmin || row.OwnerUserID == user.ID
}

func (h *Handler) listFactoryPlans(ctx context.Context, user auth.User, includeArchived bool, statusFilters []string) ([]map[string]any, error) {
	query := `
		SELECT fp.id, fp.owner_user_id, u.username, fp.name, fp.visibility, fp.status,
			fp.target_item_class, fp.target_rate, fp.updated_at
		FROM factory_plans fp
		JOIN users u ON u.id = fp.owner_user_id
		WHERE 1=1`
	var args []any

	if user.Role == auth.RoleAdmin {
		// Admin sees all plans including others' private.
	} else {
		query += ` AND (fp.owner_user_id = ? OR fp.visibility = 'shared')`
		args = append(args, user.ID)
	}

	if !includeArchived {
		query += ` AND fp.status != 'archived'`
	}
	if len(statusFilters) > 0 {
		placeholders := make([]string, len(statusFilters))
		for i, s := range statusFilters {
			placeholders[i] = "?"
			args = append(args, s)
		}
		query += ` AND fp.status IN (` + strings.Join(placeholders, ",") + `)`
	}
	query += ` ORDER BY fp.updated_at DESC`

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rowsData []factoryPlanRow
	for rows.Next() {
		var (
			id            int64
			ownerID       int64
			ownerUsername string
			name          string
			visibility    string
			status        string
			targetItem    sql.NullString
			targetRate    sql.NullFloat64
			updatedAt     string
		)
		if err := rows.Scan(&id, &ownerID, &ownerUsername, &name, &visibility, &status, &targetItem, &targetRate, &updatedAt); err != nil {
			return nil, err
		}
		rowsData = append(rowsData, factoryPlanRow{
			ID: id, OwnerUserID: ownerID, OwnerUsername: ownerUsername,
			Name: name, Visibility: visibility, Status: status, UpdatedAt: updatedAt,
			TargetItemClass: targetItem, TargetRate: targetRate,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0)
	for _, row := range rowsData {
		if !canReadPlan(user, row) {
			continue
		}
		item, err := h.planSummaryJSON(ctx, user, row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (h *Handler) getFactoryPlan(ctx context.Context, planID int64, user auth.User) (map[string]any, error) {
	row, err := h.loadPlanRow(ctx, planID)
	if err != nil {
		return nil, err
	}
	if !canReadPlan(user, row) {
		return nil, errPlanForbidden
	}

	out, err := h.planSummaryJSON(ctx, user, row)
	if err != nil {
		return nil, err
	}

	var graph planner.PlanGraph
	if err := json.Unmarshal([]byte(row.GraphJSON), &graph); err != nil {
		return nil, err
	}
	out["graph"] = graph
	out["hasBaseline"] = row.BaselineJSON.Valid && row.BaselineJSON.String != ""

	var solverOptions map[string]any
	if row.SolverOptionsJSON != "" {
		_ = json.Unmarshal([]byte(row.SolverOptionsJSON), &solverOptions)
	}
	if solverOptions == nil {
		solverOptions = map[string]any{}
	}
	out["solverOptions"] = solverOptions
	return out, nil
}

func (h *Handler) planSummaryJSON(ctx context.Context, user auth.User, row factoryPlanRow) (map[string]any, error) {
	lock, err := planner.LoadLockState(ctx, h.db, row.ID, user.ID)
	if err != nil {
		return nil, err
	}

	canManage := canManagePlan(user, row)
	canEdit := canReadPlan(user, row) && row.Status != "archived" && lock.Held && lock.Mine

	out := map[string]any{
		"id":         row.ID,
		"name":       row.Name,
		"visibility": row.Visibility,
		"status":     planStatusToAPI(row.Status),
		"owner": map[string]any{
			"id":       row.OwnerUserID,
			"username": row.OwnerUsername,
		},
		"updatedAt": row.UpdatedAt,
		"lock":      lock,
		"canEdit":   canEdit,
		"canManage": canManage,
	}
	if user.Role == auth.RoleAdmin && row.Visibility == "private" && row.OwnerUserID != user.ID {
		out["visibilityLabel"] = "private (user)"
	}
	if row.TargetItemClass.Valid {
		out["targetItemClass"] = row.TargetItemClass.String
	}
	if row.TargetRate.Valid {
		out["targetRate"] = row.TargetRate.Float64
	}
	return out, nil
}

func planStatusToAPI(dbStatus string) string {
	if dbStatus == "in_progress" {
		return "inProgress"
	}
	return dbStatus
}

func planStatusFromAPI(apiStatus string) (string, error) {
	switch apiStatus {
	case "planning":
		return "planning", nil
	case "inProgress":
		return "in_progress", nil
	case "completed":
		return "completed", nil
	case "archived":
		return "archived", nil
	default:
		return "", fmt.Errorf("invalid status")
	}
}

func parsePlanStatusFilters(values []string) []string {
	var out []string
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			db, err := planStatusFromAPI(part)
			if err != nil {
				return nil
			}
			out = append(out, db)
		}
	}
	return out
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
