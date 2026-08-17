package poller_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"factorymate/internal/db"
	"factorymate/internal/frm"
	"factorymate/internal/notify"
	"factorymate/internal/poller"
)

func TestFirstPollBaselineNoEvents(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	phases, err := poller.LoadElevatorPhases("data/elevator_phases.json")
	if err != nil {
		t.Fatalf("load phases: %v", err)
	}

	result := firstObservationFixture(t)
	engine := &poller.Engine{DB: database, ElevatorPhases: phases}

	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	events, err := engine.PollOnce(ctx, result, now)
	if err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events on first poll, got %d: %+v", len(events), events)
	}

	assertServerState(t, ctx, database, true)
	assertPlayerBaseline(t, ctx, database, "player-1", true)
	assertCircuitBaseline(t, ctx, database, 1, true)
	assertSchematicBaseline(t, ctx, database, "milestone-1", true)
	assertResearchBaseline(t, ctx, database, "node-1", "Purchased", 4, 0, "[]")
	assertElevatorPhase(t, ctx, database, "elevator-1", 2)
}

func TestElevatorPhaseLookup(t *testing.T) {
	phases, err := poller.LoadElevatorPhases("../../data/elevator_phases.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	phase2 := []frm.PhaseItem{
		{ClassName: "Desc_SpaceElevatorPart_3_C"},
		{ClassName: "Desc_SpaceElevatorPart_1_C"},
		{ClassName: "Desc_SpaceElevatorPart_2_C"},
	}
	phase, ok := phases.LookupPhase(phase2)
	if !ok || phase != 2 {
		t.Fatalf("LookupPhase phase2 = %d, %v; want 2, true", phase, ok)
	}

	unknown := []frm.PhaseItem{{ClassName: "Desc_ModdedPart_C"}}
	_, ok = phases.LookupPhase(unknown)
	if ok {
		t.Fatal("expected unknown phase lookup to fail")
	}
}

func TestElevatorUnknownPhaseLogsWithDedup(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	phases, err := poller.LoadElevatorPhases("data/elevator_phases.json")
	if err != nil {
		t.Fatalf("load phases: %v", err)
	}

	engine := &poller.Engine{DB: database, ElevatorPhases: phases}
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	unknownPhase := []frm.PhaseItem{{Name: "Mod Part", ClassName: "Desc_ModdedPart_C"}}
	result := frm.FastPollResult{
		Elevators: []frm.Elevator{{
			ID:           "elevator-unknown",
			Name:         "Space Elevator",
			CurrentPhase: unknownPhase,
			UpgradeReady: false,
		}},
	}

	for i := 0; i < 3; i++ {
		if _, err := engine.PollOnce(ctx, result, now.Add(time.Duration(i)*20*time.Second)); err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
	}

	var count int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM elevator_phase_unknown_log WHERE resolved = 0`,
	).Scan(&count); err != nil {
		t.Fatalf("count unknown log: %v", err)
	}
	if count != 1 {
		t.Fatalf("unknown log rows = %d, want 1 (dedup)", count)
	}

	var phase sql.NullInt64
	if err := database.QueryRowContext(ctx,
		`SELECT phase_number FROM elevator_state WHERE elevator_id = 'elevator-unknown'`,
	).Scan(&phase); err != nil {
		t.Fatalf("query elevator_state: %v", err)
	}
	if phase.Valid {
		t.Fatalf("phase_number should be NULL for unknown set, got %d", phase.Int64)
	}
}

func TestServerOfflineOnUnreachable(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	phases, _ := poller.LoadElevatorPhases("data/elevator_phases.json")
	engine := &poller.Engine{DB: database, ElevatorPhases: phases}
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	// Establish reachable baseline.
	okResult := firstObservationFixture(t)
	if _, err := engine.PollOnce(ctx, okResult, now); err != nil {
		t.Fatalf("baseline poll: %v", err)
	}

	// Second poll: unreachable should fire server_offline.
	failResult := frm.FastPollResult{Errors: map[string]error{"getPlayer": errUnreachable}}
	events, err := engine.PollOnce(ctx, failResult, now.Add(20*time.Second))
	if err != nil {
		t.Fatalf("offline poll: %v", err)
	}
	if len(events) != 1 || events[0].MessageTypeKey != "server_offline" {
		t.Fatalf("expected server_offline, got %+v", events)
	}

	// Player state should remain from baseline (not cleared).
	var playerCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM player_state`).Scan(&playerCount); err != nil {
		t.Fatalf("count players: %v", err)
	}
	if playerCount == 0 {
		t.Fatal("player_state should be untouched while unreachable")
	}
}

func TestDispatchWiredInPollLoop(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	var webhookCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfgJSON, _ := json.Marshal(notify.DiscordConfig{WebhookURL: srv.URL})
	res, err := database.ExecContext(ctx, `
		INSERT INTO notification_targets (name, provider_type, config_json, enabled, created_at)
		VALUES (?, ?, ?, 1, ?)`,
		"Test", "discord", string(cfgJSON), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert target: %v", err)
	}
	targetID, _ := res.LastInsertId()
	if _, err := database.ExecContext(ctx,
		`INSERT INTO message_type_targets (message_type_key, target_id) VALUES ('player_joined', ?)`, targetID); err != nil {
		t.Fatalf("assign target: %v", err)
	}

	phases, _ := poller.LoadElevatorPhases("data/elevator_phases.json")
	dispatcher := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(),
	})

	fetcher := &sequenceFetcher{
		steps: []frm.FastPollResult{
			{Players: []frm.Player{{ID: "p1", Name: "Alice", Online: false}}},
			{Players: []frm.Player{{ID: "p1", Name: "Alice", Online: true}}},
		},
	}

	p := poller.New(database, fetcher, phases, func(ctx context.Context, ev poller.Event) error {
		return dispatcher.HandleEvent(ctx, ev.MessageTypeKey, ev.Variables)
	})

	if err := p.Poll(ctx); err != nil {
		t.Fatalf("poll 1: %v", err)
	}
	if err := p.Poll(ctx); err != nil {
		t.Fatalf("poll 2: %v", err)
	}

	if webhookCalls != 1 {
		t.Fatalf("webhook calls = %d, want 1", webhookCalls)
	}

	var logCount int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notification_log WHERE message_type_key = 'player_joined' AND success = 1`,
	).Scan(&logCount); err != nil {
		t.Fatalf("count notification_log: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("notification_log success rows = %d, want 1", logCount)
	}
}

type sequenceFetcher struct {
	steps []frm.FastPollResult
	idx   int
}

func (f *sequenceFetcher) GetFast(ctx context.Context) frm.FastPollResult {
	if f.idx >= len(f.steps) {
		return f.steps[len(f.steps)-1]
	}
	result := f.steps[f.idx]
	f.idx++
	return result
}

func TestVehicleStuckDebounce(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	phases, _ := poller.LoadElevatorPhases("data/elevator_phases.json")
	engine := &poller.Engine{DB: database, ElevatorPhases: phases}

	vehicle := frm.Vehicle{
		ID:            frm.FlexibleID("vehicle-1"),
		VehicleType:   "Tractor",
		Autopilot:     true,
		ForwardSpeed:  0,
		FuelInventory: []frm.Item{{Amount: 10}},
	}

	base := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	result := func() frm.FastPollResult {
		return frm.FastPollResult{Vehicles: []frm.Vehicle{vehicle}}
	}

	// First poll: baseline, no events.
	if _, err := engine.PollOnce(ctx, result(), base); err != nil {
		t.Fatalf("poll 1: %v", err)
	}

	// Polls 2-3: debounce building, no stuck event yet.
	for i := 1; i <= 2; i++ {
		events, err := engine.PollOnce(ctx, result(), base.Add(time.Duration(i)*20*time.Second))
		if err != nil {
			t.Fatalf("poll %d: %v", i+1, err)
		}
		for _, ev := range events {
			if ev.MessageTypeKey == "vehicle_stuck" {
				t.Fatalf("vehicle_stuck too early on poll %d", i+1)
			}
		}
	}

	// Poll 4: debounce elapsed → vehicle_stuck.
	events, err := engine.PollOnce(ctx, result(), base.Add(60*time.Second))
	if err != nil {
		t.Fatalf("poll 4: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.MessageTypeKey == "vehicle_stuck" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected vehicle_stuck after 3 consecutive low-speed polls")
	}
}

var errUnreachable = &fixtureError{msg: "connection refused"}

type fixtureError struct{ msg string }

func (e *fixtureError) Error() string { return e.msg }

func firstObservationFixture(t *testing.T) frm.FastPollResult {
	t.Helper()
	return frm.FastPollResult{
		Players: []frm.Player{{
			ID: "player-1", Name: "Alice", Online: true,
		}},
		Power: []frm.Circuit{{
			CircuitGroupID: 1, FuseTriggered: true,
		}},
		Schematics: []frm.Schematic{{
			ID: "milestone-1", Name: "HUB Upgrade 1", Type: "Milestone",
			Purchased: true, TechTier: 1,
			Recipes: []frm.Recipe{{Name: "Iron Plates"}},
		}},
		Elevators: []frm.Elevator{{
			ID:   "elevator-1",
			Name: "Space Elevator",
			CurrentPhase: []frm.PhaseItem{
				{ClassName: "Desc_SpaceElevatorPart_1_C"},
				{ClassName: "Desc_SpaceElevatorPart_2_C"},
				{ClassName: "Desc_SpaceElevatorPart_3_C"},
			},
			UpgradeReady: true,
		}},
		Research: []frm.ResearchTree{{
			Name: "Test Tree",
			Nodes: []frm.ResearchNode{{
				ID: "node-1", Name: "Test Node", State: "Purchased", TechTier: 2,
				Coordinates: frm.ResearchCoordinate{X: 4, Y: 0},
				Parents:     []frm.ResearchCoordinate{},
			}},
		}},
		Trains: []frm.Train{{
			ID: "train-1", Name: "Train 1", Derailed: true,
		}},
		Vehicles: []frm.Vehicle{{
			ID: frm.FlexibleID("vehicle-1"), VehicleType: "Tractor",
			FuelInventory: []frm.Item{{Amount: 5}},
		}},
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return database
}

func assertServerState(t *testing.T, ctx context.Context, database *sql.DB, wantOnline bool) {
	t.Helper()
	var online bool
	err := database.QueryRowContext(ctx,
		`SELECT server_online FROM server_state WHERE id = 1`).Scan(&online)
	if err != nil {
		t.Fatalf("server_state: %v", err)
	}
	if online != wantOnline {
		t.Fatalf("server_online = %v, want %v", online, wantOnline)
	}
}

func assertPlayerBaseline(t *testing.T, ctx context.Context, database *sql.DB, id string, wantOnline bool) {
	t.Helper()
	var online bool
	if err := database.QueryRowContext(ctx,
		`SELECT online FROM player_state WHERE player_id = ?`, id,
	).Scan(&online); err != nil {
		t.Fatalf("player_state: %v", err)
	}
	if online != wantOnline {
		t.Fatalf("player %s online = %v, want %v", id, online, wantOnline)
	}
}

func assertCircuitBaseline(t *testing.T, ctx context.Context, database *sql.DB, id int, wantTripped bool) {
	t.Helper()
	var tripped bool
	if err := database.QueryRowContext(ctx,
		`SELECT tripped FROM circuit_state WHERE circuit_id = ?`, id,
	).Scan(&tripped); err != nil {
		t.Fatalf("circuit_state: %v", err)
	}
	if tripped != wantTripped {
		t.Fatalf("circuit %d tripped = %v, want %v", id, tripped, wantTripped)
	}
}

func assertSchematicBaseline(t *testing.T, ctx context.Context, database *sql.DB, id string, wantPurchased bool) {
	t.Helper()
	var purchased bool
	if err := database.QueryRowContext(ctx,
		`SELECT purchased FROM schematic_state WHERE schematic_id = ?`, id,
	).Scan(&purchased); err != nil {
		t.Fatalf("schematic_state: %v", err)
	}
	if purchased != wantPurchased {
		t.Fatalf("schematic %s purchased = %v, want %v", id, purchased, wantPurchased)
	}
}

func assertResearchBaseline(t *testing.T, ctx context.Context, database *sql.DB, id, wantState string, wantX, wantY int, wantParentsJSON string) {
	t.Helper()
	var state string
	var coordX, coordY int
	var parentsJSON string
	if err := database.QueryRowContext(ctx,
		`SELECT state, coord_x, coord_y, parents_json FROM research_node_state WHERE node_id = ?`, id,
	).Scan(&state, &coordX, &coordY, &parentsJSON); err != nil {
		t.Fatalf("research_node_state: %v", err)
	}
	if state != wantState {
		t.Fatalf("node %s state = %q, want %q", id, state, wantState)
	}
	if coordX != wantX || coordY != wantY {
		t.Fatalf("node %s coords = (%d,%d), want (%d,%d)", id, coordX, coordY, wantX, wantY)
	}
	if parentsJSON != wantParentsJSON {
		t.Fatalf("node %s parents_json = %q, want %q", id, parentsJSON, wantParentsJSON)
	}
}

func assertElevatorPhase(t *testing.T, ctx context.Context, database *sql.DB, id string, wantPhase int) {
	t.Helper()
	var phase int
	if err := database.QueryRowContext(ctx,
		`SELECT phase_number FROM elevator_state WHERE elevator_id = ?`, id,
	).Scan(&phase); err != nil {
		t.Fatalf("elevator_state: %v", err)
	}
	if phase != wantPhase {
		t.Fatalf("elevator %s phase = %d, want %d", id, phase, wantPhase)
	}
}

func loadFixture(t *testing.T, name string, dest any) {
	t.Helper()
	path := filepath.Join("testdata", "frm", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
}
