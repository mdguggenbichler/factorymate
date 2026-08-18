package registration_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"factorymate/internal/db"
	"factorymate/internal/registration"
)

func TestDeriveUsername(t *testing.T) {
	tests := []struct {
		display, discord, want string
	}{
		{"Michael", "mike_discord", "michael"},
		{"", "Mike_Discord", "mike_discord"},
		{"  Hello World!  ", "x", "hello-world"},
		{"!!!", "valid_user", "valid_user"},
		{"A very long display name that exceeds thirty two characters", "x", "a-very-long-display-name-that-ex"},
	}
	for _, tc := range tests {
		got := registration.DeriveUsername(tc.display, tc.discord)
		if got != tc.want {
			t.Errorf("DeriveUsername(%q, %q) = %q, want %q", tc.display, tc.discord, got, tc.want)
		}
	}
}

func TestAllocateUsername(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openUsernameTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	first, err := registration.AllocateUsername(ctx, database, "michael")
	if err != nil || first != "michael" {
		t.Fatalf("first = %q, err = %v", first, err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO users (username, password_hash, role, created_at) VALUES ('michael', 'hash', 'viewer', 'now')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	second, err := registration.AllocateUsername(ctx, database, "michael")
	if err != nil || second != "michael-2" {
		t.Fatalf("second = %q, err = %v", second, err)
	}
}

func openUsernameTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database
}
