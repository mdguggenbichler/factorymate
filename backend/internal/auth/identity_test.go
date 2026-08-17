package auth_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/db"
)

func TestTryResolvePendingPlayers(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openAuthTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, created_at, pending_player_name, status)
		VALUES ('player-user', 'hash', 'viewer', '2026-01-01T00:00:00Z', 'Guggi', 'active')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO player_state (player_id, name, online, last_seen_at)
		VALUES ('pid-1', 'Guggi', 1, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert player: %v", err)
	}

	if err := auth.TryResolvePendingPlayers(ctx, database, "pid-1", "Guggi"); err != nil {
		t.Fatalf("resolve pending: %v", err)
	}

	var playerID string
	if err := database.QueryRowContext(ctx, `SELECT player_id FROM users WHERE username = 'player-user'`).Scan(&playerID); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if playerID != "pid-1" {
		t.Fatalf("player_id = %q, want pid-1", playerID)
	}
}

func openAuthTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database
}
