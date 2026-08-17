package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"factorymate/internal/frm"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var serverOnline sql.NullBool
	var serverName string
	err := h.db.QueryRowContext(ctx, `
		SELECT ss.server_online, a.server_name
		FROM app_settings a
		LEFT JOIN server_state ss ON ss.id = 1
		WHERE a.id = 1`,
	).Scan(&serverOnline, &serverName)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	var onlineCount int
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM player_state WHERE online = 1`,
	).Scan(&onlineCount); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	rows, err := h.db.QueryContext(ctx,
		`SELECT circuit_id FROM circuit_state WHERE tripped = 1 ORDER BY circuit_id`,
	)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	tripped := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		tripped = append(tripped, id)
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	latestMilestone, err := h.queryLatestMilestone(ctx)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	elevatorSummary, err := h.queryElevatorSummary(ctx)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	out := map[string]any{
		"serverOnline":      serverOnline.Valid && serverOnline.Bool,
		"serverName":        serverName,
		"onlinePlayerCount": onlineCount,
		"trippedCircuits":   tripped,
		"latestMilestone":   latestMilestone,
		"elevator":          elevatorSummary,
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) queryLatestMilestone(ctx context.Context) (any, error) {
	var name string
	var techTier int
	var purchasedAt string
	err := h.db.QueryRowContext(ctx, `
		SELECT name, tech_tier, purchased_at
		FROM schematic_state
		WHERE type = 'Milestone' AND purchased = 1 AND purchased_at IS NOT NULL
		ORDER BY purchased_at DESC
		LIMIT 1`,
	).Scan(&name, &techTier, &purchasedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":       name,
		"techTier":   techTier,
		"unlockedAt": purchasedAt,
	}, nil
}

func (h *Handler) queryElevatorSummary(ctx context.Context) (map[string]any, error) {
	var elevatorID, name string
	var upgradeReady bool
	var phaseNumber sql.NullInt64
	var phaseJSON string
	err := h.db.QueryRowContext(ctx, `
		SELECT elevator_id, name, upgrade_ready, phase_number, current_phase_json
		FROM elevator_state LIMIT 1`,
	).Scan(&elevatorID, &name, &upgradeReady, &phaseNumber, &phaseJSON)
	if err == sql.ErrNoRows {
		return map[string]any{
			"name":            "",
			"phaseNumber":     nil,
			"upgradeReady":    false,
			"percentComplete": nil,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	summary := map[string]any{
		"name":            name,
		"upgradeReady":    upgradeReady,
		"phaseNumber":     nullIntPtr(phaseNumber),
		"percentComplete": elevatorPercentComplete(phaseJSON),
	}
	return summary, nil
}

func elevatorPercentComplete(phaseJSON string) *float64 {
	var items []frm.PhaseItem
	if err := json.Unmarshal([]byte(phaseJSON), &items); err != nil || len(items) == 0 {
		return nil
	}
	var remaining, total int
	for _, item := range items {
		remaining += item.RemainingCost
		total += item.TotalCost
	}
	if total == 0 {
		return nil
	}
	pct := 100.0 * (1.0 - float64(remaining)/float64(total))
	return &pct
}

func (h *Handler) GetPlayers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT player_id, name, online, last_seen_at
		FROM player_state ORDER BY name`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	players := make([]map[string]any, 0)
	for rows.Next() {
		var id, name string
		var online bool
		var lastSeen sql.NullString
		if err := rows.Scan(&id, &name, &online, &lastSeen); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		players = append(players, map[string]any{
			"id":         id,
			"name":       name,
			"online":     online,
			"lastSeenAt": nullStringPtr(lastSeen),
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"players": players})
}

func (h *Handler) GetPlayersHistory(w http.ResponseWriter, r *http.Request) {
	p := parsePagination(r)
	var total int
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM player_session_events`,
	).Scan(&total); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, player_id, player_name, event_type, online_count, occurred_at
		FROM player_session_events
		ORDER BY occurred_at DESC
		LIMIT ? OFFSET ?`, p.Limit, p.Offset)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var playerID, playerName, eventType string
		var onlineCount int
		var occurredAt string
		if err := rows.Scan(&id, &playerID, &playerName, &eventType, &onlineCount, &occurredAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		items = append(items, map[string]any{
			"id":          id,
			"playerId":    playerID,
			"playerName":  playerName,
			"eventType":   eventType,
			"onlineCount": onlineCount,
			"occurredAt":  occurredAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) GetPower(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT circuit_id, tripped, power_capacity, power_production, power_consumed,
			power_max_consumed, battery_differential, battery_percent, battery_capacity,
			battery_time_empty, battery_time_full, updated_at
		FROM circuit_state ORDER BY circuit_id`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	circuits := make([]map[string]any, 0)
	for rows.Next() {
		var circuitID int
		var tripped bool
		var powerCapacity, powerProduction, powerConsumed, powerMaxConsumed sql.NullFloat64
		var batteryDifferential, batteryPercent, batteryCapacity sql.NullFloat64
		var batteryTimeEmpty, batteryTimeFull sql.NullString
		var updatedAt string
		if err := rows.Scan(
			&circuitID, &tripped, &powerCapacity, &powerProduction, &powerConsumed,
			&powerMaxConsumed, &batteryDifferential, &batteryPercent, &batteryCapacity,
			&batteryTimeEmpty, &batteryTimeFull, &updatedAt,
		); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		circuits = append(circuits, map[string]any{
			"circuitId":           circuitID,
			"tripped":             tripped,
			"powerCapacity":       nullFloatPtr(powerCapacity),
			"powerProduction":     nullFloatPtr(powerProduction),
			"powerConsumed":       nullFloatPtr(powerConsumed),
			"powerMaxConsumed":    nullFloatPtr(powerMaxConsumed),
			"batteryDifferential": nullFloatPtr(batteryDifferential),
			"batteryPercent":      nullFloatPtr(batteryPercent),
			"batteryCapacity":     nullFloatPtr(batteryCapacity),
			"batteryTimeEmpty":    nullStringPtr(batteryTimeEmpty),
			"batteryTimeFull":     nullStringPtr(batteryTimeFull),
			"updatedAt":           updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"circuits": circuits})
}

func (h *Handler) GetPowerHistory(w http.ResponseWriter, r *http.Request) {
	p := parsePagination(r)
	var total int
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM power_circuit_events`,
	).Scan(&total); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, circuit_id, event_type, occurred_at
		FROM power_circuit_events
		ORDER BY occurred_at DESC
		LIMIT ? OFFSET ?`, p.Limit, p.Offset)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var circuitID int
		var eventType, occurredAt string
		if err := rows.Scan(&id, &circuitID, &eventType, &occurredAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		items = append(items, map[string]any{
			"id":         id,
			"circuitId":  circuitID,
			"eventType":  eventType,
			"occurredAt": occurredAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) GetPowerMetrics(w http.ResponseWriter, r *http.Request) {
	tr, err := parseTimeRange(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid date range")
		return
	}

	circuitFilter := strings.TrimSpace(r.URL.Query().Get("circuit"))
	query := `
		SELECT id, circuit_id, power_production, power_consumed, power_capacity, battery_percent, captured_at
		FROM circuit_snapshots WHERE 1=1`
	args := make([]any, 0, 4)
	if circuitFilter != "" {
		query += " AND circuit_id = ?"
		args = append(args, circuitFilter)
	}
	if tr.From != nil {
		query += " AND captured_at >= ?"
		args = append(args, tr.From.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if tr.To != nil {
		query += " AND captured_at < ?"
		args = append(args, tr.To.UTC().Format("2006-01-02T15:04:05Z"))
	}
	query += " ORDER BY captured_at ASC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var circuitID int
		var powerProduction, powerConsumed, powerCapacity float64
		var batteryPercent sql.NullFloat64
		var capturedAt string
		if err := rows.Scan(&id, &circuitID, &powerProduction, &powerConsumed, &powerCapacity, &batteryPercent, &capturedAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		items = append(items, map[string]any{
			"id":              id,
			"circuitId":       circuitID,
			"powerProduction": powerProduction,
			"powerConsumed":   powerConsumed,
			"powerCapacity":   powerCapacity,
			"batteryPercent":  nullFloatPtr(batteryPercent),
			"capturedAt":      capturedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetProduction(w http.ResponseWriter, r *http.Request) {
	tr, err := parseTimeRange(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid date range")
		return
	}

	itemFilter := strings.TrimSpace(r.URL.Query().Get("item"))
	query := `
		SELECT id, item_class_name, item_display_name, produced_per_min, consumed_per_min, captured_at
		FROM production_snapshots WHERE 1=1`
	args := make([]any, 0, 4)
	if itemFilter != "" {
		query += " AND item_class_name = ?"
		args = append(args, itemFilter)
	}
	if tr.From != nil {
		query += " AND captured_at >= ?"
		args = append(args, tr.From.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if tr.To != nil {
		query += " AND captured_at < ?"
		args = append(args, tr.To.UTC().Format("2006-01-02T15:04:05Z"))
	}
	query += " ORDER BY captured_at ASC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var className, displayName string
		var produced, consumed float64
		var capturedAt string
		if err := rows.Scan(&id, &className, &displayName, &produced, &consumed, &capturedAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		items = append(items, map[string]any{
			"id":              id,
			"itemClassName":   className,
			"itemDisplayName": displayName,
			"producedPerMin":  produced,
			"consumedPerMin":  consumed,
			"capturedAt":      capturedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetProductionItems(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT DISTINCT item_class_name FROM prod_stats_state ORDER BY item_class_name`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		items = append(items, name)
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetProductionCurrent(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT item_class_name, item_display_name, prod_per_min_label, prod_percent, cons_percent,
			current_prod, max_prod, current_consumed, max_consumed, transfer_type, updated_at
		FROM prod_stats_state ORDER BY item_display_name`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var className, displayName, prodLabel, transferType, updatedAt string
		var prodPercent, consPercent, currentProd, maxProd, currentConsumed, maxConsumed float64
		if err := rows.Scan(
			&className, &displayName, &prodLabel, &prodPercent, &consPercent,
			&currentProd, &maxProd, &currentConsumed, &maxConsumed, &transferType, &updatedAt,
		); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		items = append(items, map[string]any{
			"itemClassName":    className,
			"itemDisplayName":  displayName,
			"prodPerMinLabel":  prodLabel,
			"prodPercent":      prodPercent,
			"consPercent":      consPercent,
			"currentProd":      currentProd,
			"maxProd":          maxProd,
			"currentConsumed":  currentConsumed,
			"maxConsumed":      maxConsumed,
			"transferType":     transferType,
			"updatedAt":        updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetProductionMachines(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT machine_id, building_type, recipe, manu_speed, is_configured, is_producing, is_paused,
			power_consumed, max_power_consumed, circuit_group_id, ingredients_json, production_json, updated_at
		FROM factory_machine_state ORDER BY building_type, recipe`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	machines := make([]map[string]any, 0)
	for rows.Next() {
		var machineID, buildingType, recipe string
		var manuSpeed float64
		var isConfigured, isProducing, isPaused bool
		var powerConsumed, maxPowerConsumed sql.NullFloat64
		var circuitGroupID sql.NullInt64
		var ingredientsJSON, productionJSON, updatedAt string
		if err := rows.Scan(
			&machineID, &buildingType, &recipe, &manuSpeed, &isConfigured, &isProducing, &isPaused,
			&powerConsumed, &maxPowerConsumed, &circuitGroupID, &ingredientsJSON, &productionJSON, &updatedAt,
		); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		var ingredients, production []any
		_ = parseJSONColumn(ingredientsJSON, &ingredients)
		_ = parseJSONColumn(productionJSON, &production)
		machines = append(machines, map[string]any{
			"machineId":        machineID,
			"buildingType":     buildingType,
			"recipe":           recipe,
			"manuSpeed":        manuSpeed,
			"isConfigured":     isConfigured,
			"isProducing":      isProducing,
			"isPaused":         isPaused,
			"powerConsumed":    nullFloatPtr(powerConsumed),
			"maxPowerConsumed": nullFloatPtr(maxPowerConsumed),
			"circuitGroupId":   nullIntPtr(circuitGroupID),
			"ingredients":      ingredients,
			"production":       production,
			"updatedAt":        updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"machines": machines})
}

func (h *Handler) GetResourceSink(w http.ResponseWriter, r *http.Request) {
	var numCoupon int
	var percent float64
	var pointsToCoupon, totalPoints int
	var updatedAt string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT num_coupon, percent, points_to_coupon, total_points, updated_at
		FROM resource_sink_state WHERE id = 1`,
	).Scan(&numCoupon, &percent, &pointsToCoupon, &totalPoints, &updatedAt)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]any{
			"numCoupon":      0,
			"percent":        0,
			"pointsToCoupon": 0,
			"totalPoints":    0,
			"updatedAt":      nil,
		})
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"numCoupon":      numCoupon,
		"percent":        percent,
		"pointsToCoupon": pointsToCoupon,
		"totalPoints":    totalPoints,
		"updatedAt":      updatedAt,
	})
}

func (h *Handler) GetResourceSinkHistory(w http.ResponseWriter, r *http.Request) {
	tr, err := parseTimeRange(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid date range")
		return
	}

	query := `
		SELECT id, num_coupon, percent, total_points, captured_at
		FROM resource_sink_snapshots WHERE 1=1`
	args := make([]any, 0, 2)
	if tr.From != nil {
		query += " AND captured_at >= ?"
		args = append(args, tr.From.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if tr.To != nil {
		query += " AND captured_at < ?"
		args = append(args, tr.To.UTC().Format("2006-01-02T15:04:05Z"))
	}
	query += " ORDER BY captured_at ASC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var numCoupon int
		var percent float64
		var totalPoints int
		var capturedAt string
		if err := rows.Scan(&id, &numCoupon, &percent, &totalPoints, &capturedAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		items = append(items, map[string]any{
			"id":          id,
			"numCoupon":   numCoupon,
			"percent":     percent,
			"totalPoints": totalPoints,
			"capturedAt":  capturedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetDrones(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT drone_id, home_station, paired_station, has_paired_station, current_destination,
			flying_speed, max_speed, current_flying_mode, updated_at
		FROM drone_state ORDER BY drone_id`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	drones := make([]map[string]any, 0)
	for rows.Next() {
		var droneID string
		var homeStation, pairedStation, destination, flyingMode sql.NullString
		var hasPaired bool
		var flyingSpeed, maxSpeed sql.NullFloat64
		var updatedAt string
		if err := rows.Scan(
			&droneID, &homeStation, &pairedStation, &hasPaired, &destination,
			&flyingSpeed, &maxSpeed, &flyingMode, &updatedAt,
		); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		drones = append(drones, map[string]any{
			"droneId":            droneID,
			"homeStation":        nullStringPtr(homeStation),
			"pairedStation":      nullStringPtr(pairedStation),
			"hasPairedStation":   hasPaired,
			"currentDestination": nullStringPtr(destination),
			"flyingSpeed":        nullFloatPtr(flyingSpeed),
			"maxSpeed":           nullFloatPtr(maxSpeed),
			"currentFlyingMode":  nullStringPtr(flyingMode),
			"updatedAt":          updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"drones": drones})
}

func (h *Handler) GetDoggos(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT doggo_id, name, inventory_json, updated_at FROM doggo_state ORDER BY name`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	doggos := make([]map[string]any, 0)
	for rows.Next() {
		var doggoID, inventoryJSON, updatedAt string
		var name sql.NullString
		if err := rows.Scan(&doggoID, &name, &inventoryJSON, &updatedAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		var inventory []any
		_ = parseJSONColumn(inventoryJSON, &inventory)
		doggos = append(doggos, map[string]any{
			"doggoId":   doggoID,
			"name":      nullStringPtr(name),
			"inventory": inventory,
			"updatedAt": updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doggos": doggos})
}

type milestoneGroupKey struct {
	Type     string
	TechTier int
}

func (h *Handler) GetMilestones(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT schematic_id, name, type, purchased, locked, tech_tier, recipes_json, purchased_at, updated_at
		FROM schematic_state ORDER BY type, tech_tier, name`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	groupMap := make(map[milestoneGroupKey][]map[string]any)
	for rows.Next() {
		var id, name, schematicType string
		var purchased, locked bool
		var techTier int
		var recipesJSON string
		var purchasedAt sql.NullString
		var updatedAt string
		if err := rows.Scan(&id, &name, &schematicType, &purchased, &locked, &techTier, &recipesJSON, &purchasedAt, &updatedAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		var recipes []frm.Recipe
		_ = parseJSONColumn(recipesJSON, &recipes)
		apiRecipes := make([]map[string]any, 0, len(recipes))
		for _, rec := range recipes {
			apiRecipes = append(apiRecipes, map[string]any{
				"name":      rec.Name,
				"className": rec.ClassName,
			})
		}
		key := milestoneGroupKey{Type: schematicType, TechTier: techTier}
		groupMap[key] = append(groupMap[key], map[string]any{
			"id":          id,
			"name":        name,
			"purchased":   purchased,
			"locked":      locked,
			"recipes":     apiRecipes,
			"purchasedAt": nullStringPtr(purchasedAt),
			"updatedAt":   updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	keys := make([]milestoneGroupKey, 0, len(groupMap))
	for k := range groupMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Type != keys[j].Type {
			return keys[i].Type < keys[j].Type
		}
		return keys[i].TechTier < keys[j].TechTier
	})

	groups := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		groups = append(groups, map[string]any{
			"type":       k.Type,
			"techTier":   k.TechTier,
			"schematics": groupMap[k],
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (h *Handler) GetResearch(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT node_id, tree_name, name, category, state, tech_tier, cost_json, updated_at
		FROM research_node_state ORDER BY tree_name, name`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	treeMap := make(map[string][]map[string]any)
	for rows.Next() {
		var nodeID, treeName, name string
		var category sql.NullString
		var state string
		var techTier sql.NullInt64
		var costJSON, updatedAt string
		if err := rows.Scan(&nodeID, &treeName, &name, &category, &state, &techTier, &costJSON, &updatedAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		var costItems []frm.Item
		_ = parseJSONColumn(costJSON, &costItems)
		cost := make([]map[string]any, 0, len(costItems))
		for _, item := range costItems {
			cost = append(cost, map[string]any{
				"name":   item.Name,
				"amount": item.Amount,
			})
		}
		treeMap[treeName] = append(treeMap[treeName], map[string]any{
			"id":        nodeID,
			"name":      name,
			"category":  nullStringPtr(category),
			"state":     state,
			"techTier":  nullIntPtr(techTier),
			"cost":      cost,
			"updatedAt": updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	treeNames := make([]string, 0, len(treeMap))
	for name := range treeMap {
		treeNames = append(treeNames, name)
	}
	sort.Strings(treeNames)

	trees := make([]map[string]any, 0, len(treeNames))
	for _, name := range treeNames {
		trees = append(trees, map[string]any{
			"name":  name,
			"nodes": treeMap[name],
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"trees": trees})
}

func (h *Handler) GetVehicles(w http.ResponseWriter, r *http.Request) {
	trainRows, err := h.db.QueryContext(r.Context(), `
		SELECT train_id, name, derailed, pending_derail, status, self_driving_error,
			docking_status, path_status, station, updated_at
		FROM train_state ORDER BY name`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer trainRows.Close()

	trains := make([]map[string]any, 0)
	for trainRows.Next() {
		var trainID string
		var name sql.NullString
		var derailed, pendingDerail bool
		var status, selfDriving, docking, path, station sql.NullString
		var updatedAt string
		if err := trainRows.Scan(
			&trainID, &name, &derailed, &pendingDerail, &status, &selfDriving,
			&docking, &path, &station, &updatedAt,
		); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		trains = append(trains, map[string]any{
			"trainId":           trainID,
			"name":              nullStringPtr(name),
			"derailed":          derailed,
			"pendingDerail":     pendingDerail,
			"status":            nullStringPtr(status),
			"selfDrivingError":  nullStringPtr(selfDriving),
			"dockingStatus":     nullStringPtr(docking),
			"pathStatus":        nullStringPtr(path),
			"station":           nullStringPtr(station),
			"updatedAt":         updatedAt,
		})
	}
	if err := trainRows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	vehicleRows, err := h.db.QueryContext(r.Context(), `
		SELECT vehicle_id, vehicle_type, display_name, status, driver, autopilot,
			following_path, forward_speed, fuel_empty, low_speed_since, stuck, updated_at
		FROM vehicle_state ORDER BY display_name`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer vehicleRows.Close()

	vehicles := make([]map[string]any, 0)
	for vehicleRows.Next() {
		var vehicleID, vehicleType, displayName string
		var status, driver sql.NullString
		var autopilot, followingPath, fuelEmpty, stuck bool
		var forwardSpeed sql.NullFloat64
		var lowSpeedSince sql.NullString
		var updatedAt string
		if err := vehicleRows.Scan(
			&vehicleID, &vehicleType, &displayName, &status, &driver, &autopilot,
			&followingPath, &forwardSpeed, &fuelEmpty, &lowSpeedSince, &stuck, &updatedAt,
		); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		vehicles = append(vehicles, map[string]any{
			"vehicleId":     vehicleID,
			"vehicleType":   vehicleType,
			"displayName":   displayName,
			"status":        nullStringPtr(status),
			"driver":        nullStringPtr(driver),
			"autopilot":     autopilot,
			"followingPath": followingPath,
			"forwardSpeed":  nullFloatPtr(forwardSpeed),
			"fuelEmpty":     fuelEmpty,
			"lowSpeedSince": nullStringPtr(lowSpeedSince),
			"stuck":         stuck,
			"updatedAt":     updatedAt,
		})
	}
	if err := vehicleRows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"trains": trains, "vehicles": vehicles})
}

func (h *Handler) GetElevator(w http.ResponseWriter, r *http.Request) {
	var elevatorID, name string
	var upgradeReady bool
	var phaseNumber sql.NullInt64
	var phaseJSON, updatedAt string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT elevator_id, name, upgrade_ready, phase_number, current_phase_json, updated_at
		FROM elevator_state LIMIT 1`,
	).Scan(&elevatorID, &name, &upgradeReady, &phaseNumber, &phaseJSON, &updatedAt)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]any{
			"elevatorId":    "",
			"name":          "",
			"upgradeReady":  false,
			"phaseNumber":   nil,
			"currentPhase":  []any{},
			"updatedAt":     nil,
		})
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	var rawPhase []frm.PhaseItem
	_ = parseJSONColumn(phaseJSON, &rawPhase)
	currentPhase := make([]map[string]any, 0, len(rawPhase))
	for _, item := range rawPhase {
		currentPhase = append(currentPhase, map[string]any{
			"name":          item.Name,
			"className":     item.ClassName,
			"amount":        item.Amount,
			"remainingCost": item.RemainingCost,
			"totalCost":     item.TotalCost,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"elevatorId":    elevatorID,
		"name":          name,
		"upgradeReady":  upgradeReady,
		"phaseNumber":   nullIntPtr(phaseNumber),
		"currentPhase":  currentPhase,
		"updatedAt":     updatedAt,
	})
}

func (h *Handler) GetElevatorUnknownLog(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, raw_current_phase_json, detected_at, resolved, resolved_at
		FROM elevator_phase_unknown_log
		WHERE resolved = 0 OR (resolved = 1 AND resolved_at >= datetime('now', '-30 days'))
		ORDER BY detected_at DESC
		LIMIT 50`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var rawJSON, detectedAt string
		var resolved bool
		var resolvedAt sql.NullString
		if err := rows.Scan(&id, &rawJSON, &detectedAt, &resolved, &resolvedAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		var rawPhase []any
		_ = parseJSONColumn(rawJSON, &rawPhase)
		items = append(items, map[string]any{
			"id":           id,
			"currentPhase": rawPhase,
			"detectedAt":   detectedAt,
			"resolved":     resolved,
			"resolvedAt":   nullStringPtr(resolvedAt),
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ResolveElevatorUnknownLog(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	now := timeNowRFC3339()
	res, err := h.db.ExecContext(r.Context(), `
		UPDATE elevator_phase_unknown_log
		SET resolved = 1, resolved_at = ?
		WHERE id = ? AND resolved = 0`, now, id)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
