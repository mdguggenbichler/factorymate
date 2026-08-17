package notify_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"factorymate/internal/db"
	"factorymate/internal/notify"
	"factorymate/internal/template"
)

func TestDispatcher_PlayerJoined(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openDispatchTestDB(t)
	defer database.Close()

	var gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Embeds []struct {
				Title string `json:"title"`
			} `json:"embeds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Embeds) != 1 {
			t.Fatalf("embeds len = %d, want 1", len(payload.Embeds))
		}
		gotTitle = payload.Embeds[0].Title
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	targetID := insertDiscordTarget(t, ctx, database, srv.URL)
	if err := assignTarget(t, ctx, database, "player_joined", targetID); err != nil {
		t.Fatalf("assign target: %v", err)
	}

	fixedNow := time.Date(2026, 8, 17, 14, 0, 0, 0, time.UTC)
	d := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(),
	})
	d.Now = func() time.Time { return fixedNow }

	err := d.HandleEvent(ctx, "player_joined", map[string]string{
		"PlayerName":  "Guggi",
		"OnlineCount": "3",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	if gotTitle != "🟢 NEW PLAYER DETECTED" {
		t.Fatalf("webhook title = %q, want player joined embed title", gotTitle)
	}

	var preview string
	var success bool
	var errText sql.NullString
	var sentAt string
	if err := database.QueryRowContext(ctx, `
		SELECT rendered_preview, success, error, sent_at
		FROM notification_log WHERE message_type_key = 'player_joined' AND target_id = ?`,
		targetID,
	).Scan(&preview, &success, &errText, &sentAt); err != nil {
		t.Fatalf("query notification_log: %v", err)
	}
	if !success {
		t.Fatalf("notification_log success = false, error = %v", errText)
	}
	if preview == "" {
		t.Fatal("rendered_preview should not be empty")
	}
	if sentAt != fixedNow.UTC().Format(time.RFC3339) {
		t.Fatalf("sent_at = %q, want %q", sentAt, fixedNow.UTC().Format(time.RFC3339))
	}
}

func TestDispatcher_SkipsDisabledMessageType(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openDispatchTestDB(t)
	defer database.Close()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	targetID := insertDiscordTarget(t, ctx, database, srv.URL)
	if err := assignTarget(t, ctx, database, "player_joined", targetID); err != nil {
		t.Fatalf("assign target: %v", err)
	}
	if _, err := database.ExecContext(ctx,
		`UPDATE message_types SET enabled = 0 WHERE key = 'player_joined'`); err != nil {
		t.Fatalf("disable message type: %v", err)
	}

	d := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(),
	})
	if err := d.HandleEvent(ctx, "player_joined", map[string]string{
		"PlayerName":  "Guggi",
		"OnlineCount": "3",
	}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if called {
		t.Fatal("webhook should not be called when message type is disabled")
	}

	var count int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notification_log WHERE message_type_key = 'player_joined'`,
	).Scan(&count); err != nil {
		t.Fatalf("count notification_log: %v", err)
	}
	if count != 0 {
		t.Fatalf("notification_log rows = %d, want 0 for disabled type", count)
	}
}

func TestDispatcher_SkipsDisabledTarget(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openDispatchTestDB(t)
	defer database.Close()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	targetID := insertDiscordTarget(t, ctx, database, srv.URL)
	if err := assignTarget(t, ctx, database, "player_joined", targetID); err != nil {
		t.Fatalf("assign target: %v", err)
	}
	if _, err := database.ExecContext(ctx,
		`UPDATE notification_targets SET enabled = 0 WHERE id = ?`, targetID); err != nil {
		t.Fatalf("disable target: %v", err)
	}

	d := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(),
	})
	if err := d.HandleEvent(ctx, "player_joined", map[string]string{
		"PlayerName":  "Guggi",
		"OnlineCount": "3",
	}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if called {
		t.Fatal("webhook should not be called when target is disabled")
	}
}

func TestDispatcher_LogsSendFailure(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openDispatchTestDB(t)
	defer database.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid webhook"))
	}))
	defer srv.Close()

	targetID := insertDiscordTarget(t, ctx, database, srv.URL)
	if err := assignTarget(t, ctx, database, "player_joined", targetID); err != nil {
		t.Fatalf("assign target: %v", err)
	}

	d := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(),
	})
	if err := d.HandleEvent(ctx, "player_joined", map[string]string{
		"PlayerName":  "Guggi",
		"OnlineCount": "3",
	}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	var success bool
	var errText sql.NullString
	if err := database.QueryRowContext(ctx, `
		SELECT success, error FROM notification_log WHERE target_id = ?`, targetID,
	).Scan(&success, &errText); err != nil {
		t.Fatalf("query notification_log: %v", err)
	}
	if success {
		t.Fatal("expected success = false for failed webhook")
	}
	if !errText.Valid || errText.String == "" {
		t.Fatal("expected error text in notification_log")
	}
}

func TestDispatcher_SendRenderedTest(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openDispatchTestDB(t)
	defer database.Close()

	var gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Embeds []struct {
				Title string `json:"title"`
			} `json:"embeds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Embeds) != 1 {
			t.Fatalf("embeds len = %d, want 1", len(payload.Embeds))
		}
		gotTitle = payload.Embeds[0].Title
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	targetID := insertDiscordTarget(t, ctx, database, srv.URL)
	if err := assignTarget(t, ctx, database, "player_joined", targetID); err != nil {
		t.Fatalf("assign target: %v", err)
	}
	if _, err := database.ExecContext(ctx,
		`UPDATE message_types SET enabled = 0 WHERE key = 'player_joined'`); err != nil {
		t.Fatalf("disable message type: %v", err)
	}

	d := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(),
	})

	rendered := template.Render(template.Template{
		Embed: &template.EmbedTemplate{Title: "Test Title"},
	}, template.SampleVariables("player_joined"))

	if err := d.SendRenderedTest(ctx, "player_joined", rendered); err != nil {
		t.Fatalf("SendRenderedTest: %v", err)
	}
	if gotTitle != "Test Title" {
		t.Fatalf("webhook title = %q, want Test Title", gotTitle)
	}

	if err := d.SendRenderedTest(ctx, "player_left", rendered); err != notify.ErrNoTargets {
		t.Fatalf("SendRenderedTest no targets = %v, want ErrNoTargets", err)
	}
}

func openDispatchTestDB(t *testing.T) *sql.DB {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Init(context.Background(), database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	return database
}

func insertDiscordTarget(t *testing.T, ctx context.Context, database *sql.DB, webhookURL string) int64 {
	t.Helper()
	cfgJSON, err := json.Marshal(notify.DiscordConfig{WebhookURL: webhookURL})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	res, err := database.ExecContext(ctx, `
		INSERT INTO notification_targets (name, provider_type, config_json, enabled, created_at)
		VALUES (?, ?, ?, 1, ?)`,
		"Test Channel", "discord", string(cfgJSON), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert target: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

func assignTarget(t *testing.T, ctx context.Context, database *sql.DB, messageTypeKey string, targetID int64) error {
	t.Helper()
	_, err := database.ExecContext(ctx, `
		INSERT INTO message_type_targets (message_type_key, target_id) VALUES (?, ?)`,
		messageTypeKey, targetID,
	)
	return err
}
