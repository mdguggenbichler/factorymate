package auth_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"factorymate/internal/auth"
	"factorymate/internal/db"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !auth.CheckPassword(hash, "secret123") {
		t.Fatal("expected password to match hash")
	}
	if auth.CheckPassword(hash, "wrong") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestSessionCreateAndLoad(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	svc := auth.NewService(database)
	user, err := svc.CreateUser(ctx, "admin", "secret123", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	sess, err := svc.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	loaded, err := svc.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if loaded.UserID != user.ID {
		t.Fatalf("user id = %d, want %d", loaded.UserID, user.ID)
	}
	if loaded.ExpiresAt.Before(time.Now().UTC().Add(29 * 24 * time.Hour)) {
		t.Fatalf("expires too soon: %v", loaded.ExpiresAt)
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
