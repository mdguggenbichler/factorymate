package registration_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/db"
	"factorymate/internal/registration"
)

func TestRegistrationFlowAutoApprove(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	authSvc := auth.NewService(database)
	regSvc := registration.NewService(database, authSvc)

	result, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "michael",
		Password:          "password123",
		PendingPlayerName: "Michael",
		External: registration.ExternalIdentity{
			Platform:    registration.PlatformDiscord,
			UserID:      "discord-1",
			Username:    "michael",
			DisplayName: "Michael",
		},
		Role: auth.RoleViewer,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if result.PendingApproval {
		t.Fatal("expected auto-approved registration")
	}
	if result.User.Status != auth.StatusActive {
		t.Fatalf("status = %q, want active", result.User.Status)
	}
	if result.User.External.UserID == nil || *result.User.External.UserID != "discord-1" {
		t.Fatalf("external user id = %+v", result.User.External)
	}

	_, err = authSvc.Authenticate(ctx, "michael", "password123")
	if err != nil {
		t.Fatalf("login after register: %v", err)
	}
}

func TestPendingApprovalBlocksLogin(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES ('registration.auto_approve', 'false')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		t.Fatalf("disable auto approve: %v", err)
	}

	authSvc := auth.NewService(database)
	regSvc := registration.NewService(database, authSvc)

	result, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "pending-user",
		Password:          "password123",
		PendingPlayerName: "PendingPlayer",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "discord-pending",
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !result.PendingApproval {
		t.Fatal("expected pending approval")
	}

	_, err = authSvc.Authenticate(ctx, "pending-user", "password123")
	if err == nil {
		t.Fatal("expected login to be blocked for pending approval")
	}
	if err != auth.ErrPendingApproval {
		t.Fatalf("login err = %v, want ErrPendingApproval", err)
	}
}

func TestApproveAndRejectRegistration(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES ('registration.auto_approve', 'false')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		t.Fatalf("disable auto approve: %v", err)
	}

	authSvc := auth.NewService(database)
	regSvc := registration.NewService(database, authSvc)
	admin, err := authSvc.CreateUser(ctx, "admin", "password123", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	result, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "approve-me",
		Password:          "password123",
		PendingPlayerName: "ApproveMe",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "discord-approve",
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	approved, err := regSvc.ApproveRegistration(ctx, result.User.ID, admin.ID)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != auth.StatusActive {
		t.Fatalf("approved status = %q", approved.Status)
	}
	if _, err := authSvc.Authenticate(ctx, "approve-me", "password123"); err != nil {
		t.Fatalf("login after approve: %v", err)
	}

	result2, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "reject-me",
		Password:          "password123",
		PendingPlayerName: "RejectMe",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "discord-reject",
		},
	})
	if err != nil {
		t.Fatalf("register reject target: %v", err)
	}
	extID, err := regSvc.RejectRegistration(ctx, result2.User.ID, admin.ID, "not a fit")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if extID != "discord-reject" {
		t.Fatalf("external id = %q", extID)
	}
	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username = 'reject-me'`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatal("rejected user row should be deleted")
	}
}

func TestRegisterUsernameAutoSuffix(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	authSvc := auth.NewService(database)
	regSvc := registration.NewService(database, authSvc)

	_, err := database.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, created_at)
		VALUES ('michael', 'hash', 'viewer', 'now')`)
	if err != nil {
		t.Fatalf("seed username: %v", err)
	}

	result, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "michael",
		Password:          "password123",
		PendingPlayerName: "Michael2",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "discord-suffix",
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if result.User.Username != "michael-2" {
		t.Fatalf("username = %q, want michael-2", result.User.Username)
	}
}

func TestClearPlayerMapping(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	authSvc := auth.NewService(database)
	regSvc := registration.NewService(database, authSvc)
	user, err := authSvc.CreateUser(ctx, "player-user", "password123", auth.RoleViewer)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, err = database.ExecContext(ctx, `
		INSERT INTO player_state (player_id, name, online, last_seen_at)
		VALUES ('player-1', 'Alice', 1, '2026-08-17T12:00:00Z')`)
	if err != nil {
		t.Fatalf("seed player: %v", err)
	}
	_, err = database.ExecContext(ctx, `
		UPDATE users SET player_id = 'player-1', pending_player_name = 'Alice', status = 'active'
		WHERE id = ?`, user.ID)
	if err != nil {
		t.Fatalf("seed mapping: %v", err)
	}

	updated, err := regSvc.ClearPlayerMapping(ctx, user.ID)
	if err != nil {
		t.Fatalf("clear mapping: %v", err)
	}
	if updated.PlayerID != nil || updated.PendingPlayerName != nil {
		t.Fatalf("expected cleared mapping, got %+v", updated)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database
}
