package connection_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/connection"
	"factorymate/internal/db"
	"factorymate/internal/notify"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database
}

func TestRedactForLog(t *testing.T) {
	password := "hunter2"
	raw := `{"game_password":"` + password + `"}`
	redacted := connection.RedactForLog(raw)
	if strings.Contains(redacted, password) {
		t.Fatalf("redacted still contains password: %s", redacted)
	}
	plain := "Password: hunter2"
	redactedPlain := connection.RedactForLog(plain)
	if strings.Contains(redactedPlain, "hunter2") {
		t.Fatalf("redacted plain still contains password: %s", redactedPlain)
	}
}

func TestSetBroadcastsToActiveLinkedUsers(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	mock := notify.NewMockDiscordSession()
	svc := connection.NewService(database, notify.NewDiscordProvider(mock))

	input := connection.UpdateInput{
		GameHost:     strPtr("play.example.com"),
		GamePort:     intPtr(7777),
		GamePassword: strPtr("secretpass"),
	}
	_, err := svc.Set(ctx, input, 1)
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	authSvc := auth.NewService(database)
	_, err = authSvc.CreateUser(ctx, "viewer1", "password123", auth.RoleViewer)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, err = database.ExecContext(ctx, `
		UPDATE users SET external_platform = 'discord', external_user_id = 'discord-1', status = 'active'
		WHERE username = 'viewer1'`)
	if err != nil {
		t.Fatalf("link user: %v", err)
	}

	_, err = authSvc.CreateUser(ctx, "pending", "password123", auth.RoleViewer)
	if err != nil {
		t.Fatalf("create pending: %v", err)
	}
	_, err = database.ExecContext(ctx, `
		UPDATE users SET external_platform = 'discord', external_user_id = 'discord-pending', status = 'pending_approval'
		WHERE username = 'pending'`)
	if err != nil {
		t.Fatalf("set pending: %v", err)
	}

	mock.ChannelCalls = nil
	mock.DMUserIDs = nil

	input2 := connection.UpdateInput{Notes: strPtr("Epic only")}
	_, err = svc.Set(ctx, input2, 1)
	if err != nil {
		t.Fatalf("set again: %v", err)
	}

	if len(mock.DMUserIDs) != 1 || mock.DMUserIDs[0] != "discord-1" {
		t.Fatalf("DM user ids = %v, want [discord-1]", mock.DMUserIDs)
	}

	var preview string
	err = database.QueryRowContext(ctx, `
		SELECT rendered_preview FROM notification_log
		WHERE message_type_key = 'connection_details_changed' ORDER BY id DESC LIMIT 1`,
	).Scan(&preview)
	if err != nil {
		t.Fatalf("query log: %v", err)
	}
	if strings.Contains(preview, "secretpass") {
		t.Fatalf("notification_log contains password: %s", preview)
	}
}

func strPtr(s string) *string { return &s }
func intPtr(n int) *int       { return &n }
