package poller

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"factorymate/internal/frm"
	"factorymate/internal/auth"
)

// OnPlayersAutoLinked is invoked after pending_player_name rows are auto-linked (M16).
var OnPlayersAutoLinked func(ctx context.Context, links []auth.ResolvedPlayerLink) error

type appSettings struct {
	ServerName                        string
	FRMHost                           string
	FRMPort                           int
	FRMAuthToken                      sql.NullString
	PollIntervalSeconds               int
	ProductionSnapshotIntervalSeconds int
	ProductionSnapshotRetentionDays   int
}

func loadAppSettings(ctx context.Context, db *sql.DB) (appSettings, error) {
	var s appSettings
	err := db.QueryRowContext(ctx, `
		SELECT server_name, frm_host, frm_port, frm_auth_token, poll_interval_seconds,
			production_snapshot_interval_seconds, production_snapshot_retention_days
		FROM app_settings WHERE id = 1`,
	).Scan(&s.ServerName, &s.FRMHost, &s.FRMPort, &s.FRMAuthToken, &s.PollIntervalSeconds,
		&s.ProductionSnapshotIntervalSeconds, &s.ProductionSnapshotRetentionDays)
	if err != nil {
		return s, fmt.Errorf("load app_settings: %w", err)
	}
	if s.PollIntervalSeconds <= 0 {
		s.PollIntervalSeconds = 20
	}
	if s.ProductionSnapshotIntervalSeconds <= 0 {
		s.ProductionSnapshotIntervalSeconds = 300
	}
	if s.ProductionSnapshotRetentionDays <= 0 {
		s.ProductionSnapshotRetentionDays = 30
	}
	return s, nil
}

func syncSessionFromFRM(ctx context.Context, db *sql.DB, settings appSettings) (sessionSnapshot, error) {
	var snap sessionSnapshot
	if strings.TrimSpace(settings.FRMHost) == "" {
		return snap, nil
	}
	token := ""
	if settings.FRMAuthToken.Valid {
		token = settings.FRMAuthToken.String
	}
	client := frm.NewClient(frm.Config{
		Host:  strings.TrimSpace(settings.FRMHost),
		Port:  settings.FRMPort,
		Token: token,
	})
	info, err := client.GetSessionInfo(ctx)
	if err != nil {
		return snap, err
	}
	if strings.TrimSpace(info.SessionName) != "" {
		snap.ServerName = info.SessionName
	}
	snap.InGameTime = formatInGameTime(info)
	if info.SessionName != "" && info.SessionName != settings.ServerName {
		if _, err := db.ExecContext(ctx, `
			UPDATE app_settings SET server_name = ? WHERE id = 1`, info.SessionName); err != nil {
			return snap, fmt.Errorf("update server_name: %w", err)
		}
	}
	return snap, nil
}

type serverStateRow struct {
	Exists       bool
	ServerOnline sql.NullBool
}

func loadServerState(ctx context.Context, db *sql.DB) (serverStateRow, error) {
	var row serverStateRow
	err := db.QueryRowContext(ctx, `
		SELECT server_online FROM server_state WHERE id = 1`,
	).Scan(&row.ServerOnline)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return row, fmt.Errorf("load server_state: %w", err)
	}
	row.Exists = true
	return row, nil
}

func upsertServerState(ctx context.Context, db *sql.DB, online bool, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO server_state (id, server_online, updated_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			server_online = excluded.server_online,
			updated_at = excluded.updated_at`,
		online, now.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert server_state: %w", err)
	}
	return nil
}

type playerStateRow struct {
	Exists bool
	Online bool
}

func loadPlayerState(ctx context.Context, db *sql.DB, playerID string) (playerStateRow, error) {
	var row playerStateRow
	err := db.QueryRowContext(ctx,
		`SELECT online FROM player_state WHERE player_id = ?`, playerID,
	).Scan(&row.Online)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.Exists = true
	return row, nil
}

func upsertPlayerState(ctx context.Context, db *sql.DB, p frm.Player, lastSeenAt *string, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO player_state (player_id, name, online, last_seen_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(player_id) DO UPDATE SET
			name = excluded.name,
			online = excluded.online,
			last_seen_at = COALESCE(excluded.last_seen_at, player_state.last_seen_at)`,
		p.ID, p.Name, p.Online, lastSeenAt,
	)
	if err != nil {
		return err
	}
	links, err := auth.TryResolvePendingPlayers(ctx, tx, p.ID, p.Name)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if len(links) > 0 && OnPlayersAutoLinked != nil {
		if err := OnPlayersAutoLinked(ctx, links); err != nil {
			return err
		}
	}
	return nil
}

func insertPlayerSessionEvent(ctx context.Context, db *sql.DB, playerID, playerName, eventType string, onlineCount int, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO player_session_events (player_id, player_name, event_type, online_count, occurred_at)
		VALUES (?, ?, ?, ?, ?)`,
		playerID, playerName, eventType, onlineCount, now.UTC().Format(time.RFC3339),
	)
	return err
}

type circuitStateRow struct {
	Exists  bool
	Tripped bool
}

func loadCircuitState(ctx context.Context, db *sql.DB, circuitID int) (circuitStateRow, error) {
	var row circuitStateRow
	err := db.QueryRowContext(ctx,
		`SELECT tripped FROM circuit_state WHERE circuit_id = ?`, circuitID,
	).Scan(&row.Tripped)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.Exists = true
	return row, nil
}

func upsertCircuitState(ctx context.Context, db *sql.DB, c frm.Circuit, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO circuit_state (
			circuit_id, tripped, power_capacity, power_production, power_consumed,
			power_max_consumed, battery_differential, battery_percent, battery_capacity,
			battery_time_empty, battery_time_full, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(circuit_id) DO UPDATE SET
			tripped = excluded.tripped,
			power_capacity = excluded.power_capacity,
			power_production = excluded.power_production,
			power_consumed = excluded.power_consumed,
			power_max_consumed = excluded.power_max_consumed,
			battery_differential = excluded.battery_differential,
			battery_percent = excluded.battery_percent,
			battery_capacity = excluded.battery_capacity,
			battery_time_empty = excluded.battery_time_empty,
			battery_time_full = excluded.battery_time_full,
			updated_at = excluded.updated_at`,
		c.CircuitGroupID, c.FuseTriggered, c.PowerCapacity, c.PowerProduction, c.PowerConsumed,
		c.PowerMaxConsumed, c.BatteryDifferential, c.BatteryPercent, c.BatteryCapacity,
		c.BatteryTimeEmpty, c.BatteryTimeFull, now.UTC().Format(time.RFC3339),
	)
	return err
}

func insertPowerCircuitEvent(ctx context.Context, db *sql.DB, circuitID int, eventType string, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO power_circuit_events (circuit_id, event_type, occurred_at)
		VALUES (?, ?, ?)`,
		circuitID, eventType, now.UTC().Format(time.RFC3339),
	)
	return err
}

type schematicStateRow struct {
	Exists    bool
	Purchased bool
	Locked    bool
}

func loadSchematicState(ctx context.Context, db *sql.DB, schematicID string) (schematicStateRow, error) {
	var row schematicStateRow
	err := db.QueryRowContext(ctx,
		`SELECT purchased, locked FROM schematic_state WHERE schematic_id = ?`, schematicID,
	).Scan(&row.Purchased, &row.Locked)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.Exists = true
	return row, nil
}

func upsertSchematicState(ctx context.Context, db *sql.DB, s frm.Schematic, purchasedAt *string, now time.Time) error {
	recipesJSON, err := json.Marshal(s.Recipes)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO schematic_state (
			schematic_id, name, type, purchased, locked, tech_tier, recipes_json, purchased_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(schematic_id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			purchased = excluded.purchased,
			locked = excluded.locked,
			tech_tier = excluded.tech_tier,
			recipes_json = excluded.recipes_json,
			purchased_at = COALESCE(excluded.purchased_at, schematic_state.purchased_at),
			updated_at = excluded.updated_at`,
		s.ID, s.Name, s.Type, s.Purchased, s.Locked, s.TechTier, string(recipesJSON), purchasedAt,
		now.UTC().Format(time.RFC3339),
	)
	return err
}

type elevatorStateRow struct {
	Exists       bool
	UpgradeReady bool
}

func loadElevatorState(ctx context.Context, db *sql.DB, elevatorID string) (elevatorStateRow, error) {
	var row elevatorStateRow
	err := db.QueryRowContext(ctx,
		`SELECT upgrade_ready FROM elevator_state WHERE elevator_id = ?`, elevatorID,
	).Scan(&row.UpgradeReady)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.Exists = true
	return row, nil
}

func upsertElevatorState(ctx context.Context, db *sql.DB, e frm.Elevator, phaseNumber *int, now time.Time) error {
	phaseJSON, err := json.Marshal(e.CurrentPhase)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO elevator_state (elevator_id, name, upgrade_ready, phase_number, current_phase_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(elevator_id) DO UPDATE SET
			name = excluded.name,
			upgrade_ready = excluded.upgrade_ready,
			phase_number = excluded.phase_number,
			current_phase_json = excluded.current_phase_json,
			updated_at = excluded.updated_at`,
		e.ID, e.Name, e.UpgradeReady, phaseNumber, string(phaseJSON),
		now.UTC().Format(time.RFC3339),
	)
	return err
}

func insertElevatorPhaseUnknown(ctx context.Context, db *sql.DB, rawPhaseJSON string, classNamesJSON string, now time.Time) error {
	// Dedup: check unresolved rows with same sorted ClassName set.
	rows, err := db.QueryContext(ctx, `
		SELECT id, raw_current_phase_json FROM elevator_phase_unknown_log WHERE resolved = 0`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		var items []frm.PhaseItem
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			continue
		}
		existingKey, err := SortedClassNamesJSON(items)
		if err != nil {
			continue
		}
		if existingKey == classNamesJSON {
			return nil // duplicate — skip insert
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO elevator_phase_unknown_log (raw_current_phase_json, detected_at)
		VALUES (?, ?)`,
		rawPhaseJSON, now.UTC().Format(time.RFC3339),
	)
	return err
}

type researchNodeStateRow struct {
	Exists bool
	State  string
}

func loadResearchNodeState(ctx context.Context, db *sql.DB, nodeID string) (researchNodeStateRow, error) {
	var row researchNodeStateRow
	err := db.QueryRowContext(ctx,
		`SELECT state FROM research_node_state WHERE node_id = ?`, nodeID,
	).Scan(&row.State)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.Exists = true
	return row, nil
}

func upsertResearchNodeState(ctx context.Context, db *sql.DB, treeName string, n frm.ResearchNode, now time.Time) error {
	costJSON, err := json.Marshal(n.Cost)
	if err != nil {
		return err
	}
	parentsJSON, err := json.Marshal(n.Parents)
	if err != nil {
		return err
	}
	var coordX, coordY sql.NullInt64
	if n.Coordinates != nil {
		coordX = sql.NullInt64{Int64: int64(n.Coordinates.X), Valid: true}
		coordY = sql.NullInt64{Int64: int64(n.Coordinates.Y), Valid: true}
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO research_node_state (
			node_id, tree_name, name, category, state, tech_tier, cost_json,
			coord_x, coord_y, parents_json, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET
			tree_name = excluded.tree_name,
			name = excluded.name,
			category = excluded.category,
			state = excluded.state,
			tech_tier = excluded.tech_tier,
			cost_json = excluded.cost_json,
			coord_x = excluded.coord_x,
			coord_y = excluded.coord_y,
			parents_json = excluded.parents_json,
			updated_at = excluded.updated_at`,
		n.ID, treeName, n.Name, n.Category, n.State, n.TechTier, string(costJSON),
		coordX, coordY, string(parentsJSON),
		now.UTC().Format(time.RFC3339),
	)
	return err
}

func deleteResearchNodesNotInTree(ctx context.Context, db *sql.DB, treeName string, keepIDs []string) error {
	if len(keepIDs) == 0 {
		_, err := db.ExecContext(ctx, `DELETE FROM research_node_state WHERE tree_name = ?`, treeName)
		return err
	}

	placeholders := make([]string, len(keepIDs))
	args := make([]any, 0, len(keepIDs)+1)
	args = append(args, treeName)
	for i, id := range keepIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := `DELETE FROM research_node_state WHERE tree_name = ? AND node_id NOT IN (` +
		strings.Join(placeholders, ",") + `)`
	_, err := db.ExecContext(ctx, query, args...)
	return err
}

type trainStateRow struct {
	Exists   bool
	Derailed bool
}

func loadTrainState(ctx context.Context, db *sql.DB, trainID string) (trainStateRow, error) {
	var row trainStateRow
	err := db.QueryRowContext(ctx,
		`SELECT derailed FROM train_state WHERE train_id = ?`, trainID,
	).Scan(&row.Derailed)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.Exists = true
	return row, nil
}

func upsertTrainState(ctx context.Context, db *sql.DB, t frm.Train, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO train_state (
			train_id, name, derailed, pending_derail, status, self_driving_error,
			docking_status, path_status, station, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(train_id) DO UPDATE SET
			name = excluded.name,
			derailed = excluded.derailed,
			pending_derail = excluded.pending_derail,
			status = excluded.status,
			self_driving_error = excluded.self_driving_error,
			docking_status = excluded.docking_status,
			path_status = excluded.path_status,
			station = excluded.station,
			updated_at = excluded.updated_at`,
		t.ID, t.Name, t.Derailed, t.PendingDerail, t.Status, t.SelfDriving,
		t.Docking, t.Path, t.TrainStation, now.UTC().Format(time.RFC3339),
	)
	return err
}

type vehicleStateRow struct {
	Exists        bool
	FuelEmpty     bool
	LowSpeedSince sql.NullString
	Stuck         bool
}

func loadVehicleState(ctx context.Context, db *sql.DB, vehicleID string) (vehicleStateRow, error) {
	var row vehicleStateRow
	err := db.QueryRowContext(ctx, `
		SELECT fuel_empty, low_speed_since, stuck FROM vehicle_state WHERE vehicle_id = ?`,
		vehicleID,
	).Scan(&row.FuelEmpty, &row.LowSpeedSince, &row.Stuck)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.Exists = true
	return row, nil
}

func upsertVehicleState(ctx context.Context, db *sql.DB, v frm.Vehicle, fuelEmpty bool, lowSpeedSince *string, stuck bool, now time.Time) error {
	vtype := v.Type()
	_, err := db.ExecContext(ctx, `
		INSERT INTO vehicle_state (
			vehicle_id, vehicle_type, display_name, status, driver, autopilot,
			following_path, forward_speed, fuel_empty, low_speed_since, stuck, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(vehicle_id) DO UPDATE SET
			vehicle_type = excluded.vehicle_type,
			display_name = excluded.display_name,
			status = excluded.status,
			driver = excluded.driver,
			autopilot = excluded.autopilot,
			following_path = excluded.following_path,
			forward_speed = excluded.forward_speed,
			fuel_empty = excluded.fuel_empty,
			low_speed_since = excluded.low_speed_since,
			stuck = excluded.stuck,
			updated_at = excluded.updated_at`,
		v.ID.String(), vtype, v.DisplayName(), v.Status, v.Driver, v.IsAutoPilot(),
		v.FollowingPath, v.ForwardSpeed, fuelEmpty, lowSpeedSince, stuck,
		now.UTC().Format(time.RFC3339),
	)
	return err
}

func totalFuelAmount(v frm.Vehicle) int {
	total := 0
	for _, item := range v.Fuels() {
		total += item.Amount
	}
	return total
}

func recipeNames(recipes []frm.Recipe) string {
	names := make([]string, 0, len(recipes))
	for _, r := range recipes {
		names = append(names, r.Name)
	}
	return strings.Join(names, ", ")
}

func recipeOptions(recipes []frm.Recipe) string {
	names := make([]string, 0, len(recipes))
	for _, r := range recipes {
		names = append(names, r.Name)
	}
	return strings.Join(names, "\n")
}

func countOnlinePlayers(players []frm.Player) int {
	n := 0
	for _, p := range players {
		if p.Online {
			n++
		}
	}
	return n
}

func intToString(v int) string {
	return fmt.Sprintf("%d", v)
}
