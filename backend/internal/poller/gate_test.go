package poller_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"factorymate/internal/db"
	"factorymate/internal/poller"
)

func TestGate_InitialHealthy(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	gate, err := poller.NewGate(database)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	if gate.Phase() != poller.PhaseHealthy {
		t.Fatalf("phase = %q, want healthy", gate.Phase())
	}
	if !gate.AllowFRMPoll() {
		t.Fatal("expected FRM poll allowed when healthy")
	}
}

func TestGate_InitialDownWhenServerOffline(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO server_state (id, server_online, updated_at, recovery_phase)
		VALUES (1, 0, ?, 'down')
		ON CONFLICT(id) DO UPDATE SET server_online = 0, recovery_phase = 'down', updated_at = excluded.updated_at`,
		now); err != nil {
		t.Fatalf("seed server_state: %v", err)
	}

	gate, err := poller.NewGate(database)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	if gate.Phase() != poller.PhaseDown {
		t.Fatalf("phase = %q, want down", gate.Phase())
	}
	if gate.AllowFRMPoll() {
		t.Fatal("expected FRM poll blocked when down")
	}
}

func TestGate_OnFRMFailureTransitionsToDown(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	gate, err := poller.NewGate(database)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	if err := gate.OnFRMFailure(ctx); err != nil {
		t.Fatalf("OnFRMFailure: %v", err)
	}
	if gate.Phase() != poller.PhaseDown {
		t.Fatalf("phase = %q, want down", gate.Phase())
	}

	var phase string
	if err := database.QueryRowContext(ctx, `SELECT recovery_phase FROM server_state WHERE id = 1`).Scan(&phase); err != nil {
		t.Fatalf("query recovery_phase: %v", err)
	}
	if phase != "down" {
		t.Fatalf("recovery_phase = %q, want down", phase)
	}
}

func TestGate_TCPProbeTransitionsToRecovering(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	details, _ := json.Marshal(map[string]any{
		"gameHost": "127.0.0.1",
		"gamePort": 17777,
	})
	if _, err := database.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		"connection.details_json", string(details)); err != nil {
		t.Fatalf("seed connection details: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:17777")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	gate, err := poller.NewGate(database)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	gate.SetPhase(poller.PhaseDown)

	sleep := gate.RunDownCycle(ctx)
	if sleep != 5*time.Second {
		t.Fatalf("sleep = %v, want 5s", sleep)
	}
	if gate.Phase() != poller.PhaseRecovering {
		t.Fatalf("phase = %q, want recovering", gate.Phase())
	}
	if gate.AllowFRMPoll() {
		t.Fatal("FRM poll should be blocked during recovering grace")
	}
}

func TestGate_MissingConnectionDetailsStaysDown(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	gate, err := poller.NewGate(database)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	gate.SetPhase(poller.PhaseDown)
	gate.RunDownCycle(ctx)
	if gate.Phase() != poller.PhaseDown {
		t.Fatalf("phase = %q, want down when connection details missing", gate.Phase())
	}
}

func TestGate_RecoveringGraceWait(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	gate, err := poller.NewGate(database)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	gate.SetNow(func() time.Time { return now })
	gate.SetPhase(poller.PhaseRecovering)
	gate.SetGraceUntil(now.Add(30 * time.Second))

	action, sleep := gate.RunRecoveringCycle()
	if action != poller.ActionWait {
		t.Fatalf("action = %v, want wait", action)
	}
	if sleep != 30*time.Second {
		t.Fatalf("sleep = %v, want 30s", sleep)
	}
}

func TestGate_InitialRecoveringResetsToDown(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO server_state (id, server_online, updated_at, recovery_phase)
		VALUES (1, 0, ?, 'recovering')
		ON CONFLICT(id) DO UPDATE SET server_online = 0, recovery_phase = 'recovering', updated_at = excluded.updated_at`,
		now); err != nil {
		t.Fatalf("seed server_state: %v", err)
	}

	gate, err := poller.NewGate(database)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	if gate.Phase() != poller.PhaseDown {
		t.Fatalf("phase = %q, want down after restart during recovering", gate.Phase())
	}
	var phase string
	if err := database.QueryRowContext(ctx, `SELECT recovery_phase FROM server_state WHERE id = 1`).Scan(&phase); err != nil {
		t.Fatalf("query recovery_phase: %v", err)
	}
	if phase != "down" {
		t.Fatalf("recovery_phase = %q, want down", phase)
	}
}

func TestGate_RecoveringGraceElapsedProbeSession(t *testing.T) {
	t.Chdir("../..")
	database := openTestDB(t)
	defer database.Close()
	ctx := context.Background()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	gate, err := poller.NewGate(database)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	gate.SetNow(func() time.Time { return now })
	gate.SetPhase(poller.PhaseRecovering)
	gate.SetGraceUntil(now.Add(-time.Second))

	action, sleep := gate.RunRecoveringCycle()
	if action != poller.ActionProbeSession {
		t.Fatalf("action = %v, want probe session", action)
	}
	if sleep != 0 {
		t.Fatalf("sleep = %v, want 0", sleep)
	}
}
