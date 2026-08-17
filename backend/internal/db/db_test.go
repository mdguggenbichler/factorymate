package db_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"factorymate/internal/db"
)

func TestMigrateAndSeedTwiceIsNoOp(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	cfg := db.SeedConfig{FRMHost: "192.168.178.42", FRMPort: 8889}

	if err := db.Init(ctx, database, cfg); err != nil {
		t.Fatalf("first init: %v", err)
	}

	first, err := snapshotState(ctx, database)
	if err != nil {
		t.Fatalf("snapshot after first init: %v", err)
	}

	if err := db.Init(ctx, database, cfg); err != nil {
		t.Fatalf("second init: %v", err)
	}

	second, err := snapshotState(ctx, database)
	if err != nil {
		t.Fatalf("snapshot after second init: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("second init changed database state\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestFreshDatabaseHasExpectedSchemaAndSeed(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	cfg := db.SeedConfig{FRMHost: "", FRMPort: 8080}
	if err := db.Init(ctx, database, cfg); err != nil {
		t.Fatalf("init: %v", err)
	}

	assertTables(t, ctx, database)
	assertMessageTypes(t, ctx, database)
	assertAppSettings(t, ctx, database, cfg)
}

func TestSeedDoesNotOverwriteEnabled(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	cfg := db.SeedConfig{}
	if err := db.Init(ctx, database, cfg); err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, err := database.ExecContext(ctx,
		`UPDATE message_types SET enabled = 0 WHERE key = 'player_joined'`); err != nil {
		t.Fatalf("disable player_joined: %v", err)
	}

	if err := db.Seed(ctx, database, cfg); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	var enabled int
	if err := database.QueryRowContext(ctx,
		`SELECT enabled FROM message_types WHERE key = 'player_joined'`).Scan(&enabled); err != nil {
		t.Fatalf("query enabled: %v", err)
	}
	if enabled != 0 {
		t.Fatalf("enabled = %d, want 0 after re-seed", enabled)
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

type dbSnapshot struct {
	MigrationCount int
	MessageCount   int
	AppSettings    string
	MessageTypes   map[string]string
}

func snapshotState(ctx context.Context, database *sql.DB) (dbSnapshot, error) {
	var snap dbSnapshot

	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations`).Scan(&snap.MigrationCount); err != nil {
		return snap, err
	}
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM message_types`).Scan(&snap.MessageCount); err != nil {
		return snap, err
	}
	if err := database.QueryRowContext(ctx, `
		SELECT server_name || '|' || frm_host || '|' || frm_port
		FROM app_settings WHERE id = 1`).Scan(&snap.AppSettings); err != nil {
		return snap, err
	}

	rows, err := database.QueryContext(ctx, `
		SELECT key, default_template_json || '|' || variables_json || '|' || enabled
		FROM message_types ORDER BY key`)
	if err != nil {
		return snap, err
	}
	defer rows.Close()

	snap.MessageTypes = make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return snap, err
		}
		snap.MessageTypes[key] = value
	}
	return snap, rows.Err()
}

func assertTables(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	want := []string{
		"app_settings",
		"circuit_snapshots",
		"circuit_state",
		"doggo_state",
		"drone_state",
		"elevator_phase_unknown_log",
		"elevator_state",
		"factory_machine_state",
		"invites",
		"message_templates",
		"message_type_targets",
		"message_types",
		"notification_log",
		"notification_targets",
		"player_session_events",
		"player_state",
		"power_circuit_events",
		"prod_stats_state",
		"production_snapshots",
		"research_node_state",
		"resource_sink_snapshots",
		"resource_sink_state",
		"schema_migrations",
		"schematic_state",
		"server_state",
		"sessions",
		"train_state",
		"users",
		"vehicle_state",
	}

	rows, err := database.QueryContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tables: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("table count = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tables mismatch at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func assertMessageTypes(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM message_types`).Scan(&count); err != nil {
		t.Fatalf("count message_types: %v", err)
	}
	if count != 13 {
		t.Fatalf("message_types count = %d, want 13", count)
	}

	defaultsBody, err := os.ReadFile("data/message_defaults.json")
	if err != nil {
		t.Fatalf("read message_defaults.json: %v", err)
	}
	var defaults map[string]json.RawMessage
	if err := json.Unmarshal(defaultsBody, &defaults); err != nil {
		t.Fatalf("parse message_defaults.json: %v", err)
	}

	rows, err := database.QueryContext(ctx, `
		SELECT key, default_template_json, variables_json
		FROM message_types ORDER BY key`)
	if err != nil {
		t.Fatalf("query message_types: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, templateJSON, variablesJSON string
		if err := rows.Scan(&key, &templateJSON, &variablesJSON); err != nil {
			t.Fatalf("scan message type: %v", err)
		}

		wantTemplate, ok := defaults[key]
		if !ok {
			t.Fatalf("unexpected message type key %q", key)
		}
		if normalizeJSON(templateJSON) != normalizeJSON(string(wantTemplate)) {
			t.Fatalf("default_template_json for %q does not match message_defaults.json", key)
		}

		var vars []string
		if err := json.Unmarshal([]byte(variablesJSON), &vars); err != nil {
			t.Fatalf("parse variables_json for %q: %v", key, err)
		}
		if len(vars) == 0 {
			t.Fatalf("variables_json for %q is empty", key)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate message_types: %v", err)
	}
}

func assertAppSettings(t *testing.T, ctx context.Context, database *sql.DB, cfg db.SeedConfig) {
	t.Helper()

	var serverName, frmHost string
	var frmPort int
	err := database.QueryRowContext(ctx, `
		SELECT server_name, frm_host, frm_port FROM app_settings WHERE id = 1`,
	).Scan(&serverName, &frmHost, &frmPort)
	if err != nil {
		t.Fatalf("query app_settings: %v", err)
	}

	if serverName != "Satisfactory Server" {
		t.Fatalf("server_name = %q, want %q", serverName, "Satisfactory Server")
	}
	if frmHost != cfg.FRMHost {
		t.Fatalf("frm_host = %q, want %q", frmHost, cfg.FRMHost)
	}
	if frmPort != cfg.FRMPort {
		t.Fatalf("frm_port = %d, want %d", frmPort, cfg.FRMPort)
	}
}

func normalizeJSON(raw string) string {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return string(out)
}
