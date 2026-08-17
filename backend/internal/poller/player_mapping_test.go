package poller_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"factorymate/internal/db"
	"factorymate/internal/frm"
	"factorymate/internal/poller"
)

func TestPollerAutoLinksPendingPlayer(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openPollerTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, created_at, pending_player_name, status)
		VALUES ('autolink', 'hash', 'viewer', '2026-01-01T00:00:00Z', 'NewJoiner', 'active')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	phases, err := poller.LoadElevatorPhases(poller.DefaultElevatorPhasesPath())
	if err != nil {
		t.Fatalf("load phases: %v", err)
	}

	engine := &poller.Engine{DB: database, ElevatorPhases: phases}
	result := frm.FastPollResult{
		Players: []frm.Player{{ID: "p-new", Name: "NewJoiner", Online: true}},
	}
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	if _, err := engine.PollOnce(ctx, result, now); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	var playerID sql.NullString
	if err := database.QueryRowContext(ctx, `SELECT player_id FROM users WHERE username = 'autolink'`).Scan(&playerID); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if !playerID.Valid || playerID.String != "p-new" {
		t.Fatalf("player_id = %v, want p-new", playerID)
	}
}

func openPollerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database
}
