package auth_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/db"
)

func TestUpdateUserAtomic(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	svc := auth.NewService(database)
	admin, err := svc.CreateUser(ctx, "admin", "secret123", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	viewer, err := svc.CreateUser(ctx, "viewer", "viewerpass", auth.RoleViewer)
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO player_state (player_id, name, online, last_seen_at)
		VALUES ('player-1', 'Alice', 1, '2026-08-17T12:00:00Z')`)
	if err != nil {
		t.Fatalf("seed player: %v", err)
	}

	badPlayer := "missing-player"
	badPlayerPtr := &badPlayer
	roleViewer := auth.RoleViewer
	_, err = svc.UpdateUser(ctx, admin.ID, auth.UserUpdate{
		Role:     &roleViewer,
		PlayerID: &badPlayerPtr,
	})
	if err == nil {
		t.Fatal("expected invalid player update to fail")
	}
	if !strings.Contains(err.Error(), "player not found") {
		t.Fatalf("error = %v, want player not found", err)
	}

	adminAfter, err := svc.GetUserByID(ctx, admin.ID)
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if adminAfter.Role != auth.RoleAdmin {
		t.Fatalf("role = %q, want admin after failed update", adminAfter.Role)
	}

	playerID := "player-1"
	playerIDPtr := &playerID
	roleAdmin := auth.RoleAdmin
	updated, err := svc.UpdateUser(ctx, viewer.ID, auth.UserUpdate{
		Role:     &roleAdmin,
		PlayerID: &playerIDPtr,
	})
	if err != nil {
		t.Fatalf("update viewer: %v", err)
	}
	if updated.Role != auth.RoleAdmin {
		t.Fatalf("role = %q, want admin", updated.Role)
	}
	if updated.PlayerID == nil || *updated.PlayerID != playerID {
		t.Fatalf("playerId = %v, want %s", updated.PlayerID, playerID)
	}

	weak := "short"
	_, err = svc.UpdateUser(ctx, viewer.ID, auth.UserUpdate{Password: &weak})
	if err == nil {
		t.Fatal("expected weak password to fail")
	}
	if !errors.Is(err, auth.ErrWeakPassword) {
		t.Fatalf("error = %v, want ErrWeakPassword", err)
	}
}

func TestUpdateUserClearsPlayerMapping(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	svc := auth.NewService(database)
	user, err := svc.CreateUser(ctx, "mapped", "secret123", auth.RoleViewer)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO player_state (player_id, name, online, last_seen_at)
		VALUES ('player-1', 'Alice', 1, '2026-08-17T12:00:00Z')`)
	if err != nil {
		t.Fatalf("seed player: %v", err)
	}

	playerID := "player-1"
	pending := "Alice"
	_, err = database.ExecContext(ctx, `
		UPDATE users SET player_id = ?, pending_player_name = ? WHERE id = ?`,
		playerID, pending, user.ID)
	if err != nil {
		t.Fatalf("seed mapping: %v", err)
	}

	var cleared *string
	updated, err := svc.UpdateUser(ctx, user.ID, auth.UserUpdate{PlayerID: &cleared})
	if err != nil {
		t.Fatalf("clear mapping: %v", err)
	}
	if updated.PlayerID != nil {
		t.Fatalf("playerId = %v, want nil", updated.PlayerID)
	}
	if updated.PendingPlayerName != nil {
		t.Fatalf("pendingPlayerName = %v, want nil", updated.PendingPlayerName)
	}

	var dbPlayerID, dbPending sql.NullString
	if err := database.QueryRowContext(ctx, `
		SELECT player_id, pending_player_name FROM users WHERE id = ?`, user.ID,
	).Scan(&dbPlayerID, &dbPending); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if dbPlayerID.Valid || dbPending.Valid {
		t.Fatalf("db still has mapping: player_id=%v pending=%v", dbPlayerID, dbPending)
	}
}
