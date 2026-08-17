package poller_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"factorymate/internal/db"
	"factorymate/internal/frm"
	"factorymate/internal/poller"
)

func seedCircuitState(t *testing.T, ctx context.Context, database *sql.DB, now time.Time) {
	t.Helper()
	powerFixture := loadFixtureBytes(t, "getPower.json")
	var circuits []frm.Circuit
	if err := json.Unmarshal(powerFixture, &circuits); err != nil {
		t.Fatalf("parse power fixture: %v", err)
	}
	ts := now.UTC().Format(time.RFC3339)
	for _, c := range circuits {
		_, err := database.ExecContext(ctx, `
			INSERT INTO circuit_state (
				circuit_id, tripped, power_capacity, power_production, power_consumed,
				power_max_consumed, battery_differential, battery_percent, battery_capacity,
				battery_time_empty, battery_time_full, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.CircuitGroupID, c.FuseTriggered, c.PowerCapacity, c.PowerProduction, c.PowerConsumed,
			c.PowerMaxConsumed, c.BatteryDifferential, c.BatteryPercent, c.BatteryCapacity,
			c.BatteryTimeEmpty, c.BatteryTimeFull, ts,
		)
		if err != nil {
			t.Fatalf("seed circuit_state: %v", err)
		}
	}
}

func TestSlowPollCycle(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	// Seed circuit_state for circuit_snapshots (no server_state side effects).
	seedCircuitState(t, ctx, database, now)

	slowResult := slowPollFixture(t)
	slowEngine := &poller.SlowEngine{DB: database}
	if err := slowEngine.PollOnce(ctx, slowResult, now); err != nil {
		t.Fatalf("slow poll: %v", err)
	}

	assertSlowPollTables(t, ctx, database, now)
	assertServerStateUntouched(t, ctx, database)
}

func TestSlowPollPartialFailure(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	seedCircuitState(t, ctx, database, now)

	result := slowPollFixture(t)
	result.Errors = map[string]error{"getProdStats": errUnreachable}
	result.ProdStats = nil

	slowEngine := &poller.SlowEngine{DB: database}
	if err := slowEngine.PollOnce(ctx, result, now); err != nil {
		t.Fatalf("slow poll: %v", err)
	}

	var prodCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM prod_stats_state`).Scan(&prodCount); err != nil {
		t.Fatalf("count prod_stats_state: %v", err)
	}
	if prodCount != 0 {
		t.Fatalf("prod_stats_state should be empty after getProdStats failure, got %d rows", prodCount)
	}

	var sinkCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource_sink_state`).Scan(&sinkCount); err != nil {
		t.Fatalf("count resource_sink_state: %v", err)
	}
	if sinkCount != 1 {
		t.Fatalf("resource_sink_state rows = %d, want 1", sinkCount)
	}

	var circuitSnapCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM circuit_snapshots`).Scan(&circuitSnapCount); err != nil {
		t.Fatalf("count circuit_snapshots: %v", err)
	}
	if circuitSnapCount == 0 {
		t.Fatal("circuit_snapshots should still be appended on partial failure")
	}
}

func TestSlowPollRetention(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	oldTS := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	nowTS := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)

	for _, stmt := range []string{
		`INSERT INTO production_snapshots (item_class_name, item_display_name, produced_per_min, consumed_per_min, captured_at)
		 VALUES ('old', 'Old', 1, 1, ?)`,
		`INSERT INTO resource_sink_snapshots (num_coupon, percent, total_points, captured_at)
		 VALUES (1, 1, 1, ?)`,
		`INSERT INTO circuit_snapshots (circuit_id, power_production, power_consumed, power_capacity, battery_percent, captured_at)
		 VALUES (1, 1, 1, 1, 1, ?)`,
	} {
		if _, err := database.ExecContext(ctx, stmt, oldTS); err != nil {
			t.Fatalf("seed old snapshot: %v", err)
		}
	}

	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	seedCircuitState(t, ctx, database, now)

	slowEngine := &poller.SlowEngine{DB: database}
	if err := slowEngine.PollOnce(ctx, slowPollFixture(t), now); err != nil {
		t.Fatalf("slow poll: %v", err)
	}

	for _, table := range []string{"production_snapshots", "resource_sink_snapshots", "circuit_snapshots"} {
		var oldCount int
		if err := database.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM `+table+` WHERE captured_at = ?`, oldTS,
		).Scan(&oldCount); err != nil {
			t.Fatalf("count old rows in %s: %v", table, err)
		}
		if oldCount != 0 {
			t.Fatalf("%s still has %d rows older than retention window", table, oldCount)
		}
	}

	var recentCount int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM production_snapshots WHERE captured_at = ?`, nowTS,
	).Scan(&recentCount); err != nil {
		t.Fatalf("count recent production_snapshots: %v", err)
	}
	if recentCount == 0 {
		t.Fatal("expected new production_snapshots after slow poll")
	}
}

func TestSlowPollViaMockServer(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	fixtures := map[string][]byte{
		"getProdStats":    loadFixtureBytes(t, "getProdStats.json"),
		"getResourceSink": loadFixtureBytes(t, "getResourceSink.json"),
		"getFactory":      loadFixtureBytes(t, "getFactory.json"),
		"getDrone":        loadFixtureBytes(t, "getDrone.json"),
		"getDoggo":        loadFixtureBytes(t, "getDoggo.json"),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}
		body, ok := fixtures[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	seedCircuitState(t, ctx, database, now)

	client := frm.NewClient(frm.Config{Host: "127.0.0.1", Port: parseTestPort(t, srv.URL)})
	result := client.GetSlow(ctx)
	slowEngine := &poller.SlowEngine{DB: database}
	if err := slowEngine.PollOnce(ctx, result, now); err != nil {
		t.Fatalf("slow poll via client: %v", err)
	}

	assertSlowPollTables(t, ctx, database, now)
}

func slowPollFixture(t *testing.T) frm.SlowPollResult {
	t.Helper()
	var stats []frm.ProdStat
	loadFixture(t, "getProdStats.json", &stats)

	var sinks []frm.ResourceSink
	loadFixture(t, "getResourceSink.json", &sinks)

	var machines []frm.FactoryMachine
	loadFixture(t, "getFactory.json", &machines)

	var drones []frm.Drone
	loadFixture(t, "getDrone.json", &drones)

	var doggos []frm.Doggo
	loadFixture(t, "getDoggo.json", &doggos)

	result := frm.SlowPollResult{
		ProdStats: stats,
		Factory:   machines,
		Drones:    drones,
		Doggos:    doggos,
	}
	if len(sinks) > 0 {
		result.ResourceSink = &sinks[0]
	}
	return result
}

func loadFixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "frm", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return body
}

func parseTestPort(t *testing.T, baseURL string) int {
	t.Helper()
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse test server URL %q: %v", baseURL, err)
	}
	portStr := u.Port()
	if portStr == "" {
		t.Fatalf("no port in test server URL %q", baseURL)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port from %q: %v", portStr, err)
	}
	return port
}

func assertSlowPollTables(t *testing.T, ctx context.Context, database *sql.DB, now time.Time) {
	t.Helper()
	ts := now.UTC().Format(time.RFC3339)

	var prodStats int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM prod_stats_state`).Scan(&prodStats); err != nil {
		t.Fatalf("count prod_stats_state: %v", err)
	}
	if prodStats == 0 {
		t.Fatal("prod_stats_state empty")
	}

	var prodSnaps int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM production_snapshots WHERE captured_at = ?`, ts,
	).Scan(&prodSnaps); err != nil {
		t.Fatalf("count production_snapshots: %v", err)
	}
	if prodSnaps == 0 {
		t.Fatal("production_snapshots empty")
	}

	var sinkState int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource_sink_state`).Scan(&sinkState); err != nil {
		t.Fatalf("count resource_sink_state: %v", err)
	}
	if sinkState != 1 {
		t.Fatalf("resource_sink_state rows = %d, want 1", sinkState)
	}

	var sinkSnaps int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM resource_sink_snapshots WHERE captured_at = ?`, ts,
	).Scan(&sinkSnaps); err != nil {
		t.Fatalf("count resource_sink_snapshots: %v", err)
	}
	if sinkSnaps != 1 {
		t.Fatalf("resource_sink_snapshots rows = %d, want 1", sinkSnaps)
	}

	var machines int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM factory_machine_state`).Scan(&machines); err != nil {
		t.Fatalf("count factory_machine_state: %v", err)
	}
	if machines == 0 {
		t.Fatal("factory_machine_state empty")
	}

	var ingredients string
	if err := database.QueryRowContext(ctx,
		`SELECT ingredients_json FROM factory_machine_state LIMIT 1`,
	).Scan(&ingredients); err != nil {
		t.Fatalf("read ingredients_json: %v", err)
	}
	if ingredients == "" || ingredients == "[]" {
		t.Fatalf("expected non-empty ingredients_json, got %q", ingredients)
	}

	var buildingType string
	if err := database.QueryRowContext(ctx,
		`SELECT building_type FROM factory_machine_state WHERE building_type = 'Assembler' LIMIT 1`,
	).Scan(&buildingType); err != nil {
		t.Fatalf("assembler building_type: %v", err)
	}

	var drones int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM drone_state`).Scan(&drones); err != nil {
		t.Fatalf("count drone_state: %v", err)
	}
	if drones != 1 {
		t.Fatalf("drone_state rows = %d, want 1", drones)
	}

	var doggos int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM doggo_state`).Scan(&doggos); err != nil {
		t.Fatalf("count doggo_state: %v", err)
	}
	if doggos != 1 {
		t.Fatalf("doggo_state rows = %d, want 1", doggos)
	}

	var circuitSnaps int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM circuit_snapshots WHERE captured_at = ?`, ts,
	).Scan(&circuitSnaps); err != nil {
		t.Fatalf("count circuit_snapshots: %v", err)
	}
	if circuitSnaps == 0 {
		t.Fatal("circuit_snapshots empty")
	}
}

func assertServerStateUntouched(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM server_state`).Scan(&count); err != nil {
		t.Fatalf("count server_state: %v", err)
	}
	if count != 0 {
		t.Fatalf("slow poll must not touch server_state, found %d rows", count)
	}
}
