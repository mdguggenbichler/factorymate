package discord_test

import (
	"context"
	"strings"
	"testing"

	"factorymate/internal/connection"
	"factorymate/internal/db"
	"factorymate/internal/discord"
)

func TestLogBotCommandRedactsPassword(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	detail := "updated: game_host, game_password=hunter2"
	if err := discord.LogBotCommand(ctx, database, "user-1", "connection set", true, detail); err != nil {
		t.Fatalf("log: %v", err)
	}

	var logged string
	err := database.QueryRowContext(ctx, `
		SELECT detail FROM bot_command_log WHERE command_name = 'connection set' LIMIT 1`,
	).Scan(&logged)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if strings.Contains(logged, "hunter2") {
		t.Fatalf("bot_command_log contains password: %s", logged)
	}
}

func TestConnectionChangedFieldsLogDetail(t *testing.T) {
	old := connection.Details{GameHost: "a", GamePort: 7777}
	new := connection.Details{GameHost: "b", GamePort: 7777}
	input := connection.UpdateInput{GameHost: strPtr("b")}
	fields := connection.ChangedFields(old, new, input)
	detail := "updated: " + strings.Join(fields, ", ")
	if strings.Contains(detail, "secret") {
		t.Fatal("detail should not contain password values")
	}
	if !strings.Contains(detail, "game_host") {
		t.Fatalf("detail = %q", detail)
	}
}

func strPtr(s string) *string { return &s }
