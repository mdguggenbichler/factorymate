package savegame

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"factorymate/internal/db"
)

func TestServiceDownloadLatest(t *testing.T) {
	t.Chdir("../..")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Function string `json:"function"`
		}
		_ = json.Unmarshal(body, &req)
		switch req.Function {
		case "QueryServerState":
			_, _ = w.Write([]byte(`{"data":{"serverGameState":{"activeSessionName":"Test Session","isGameRunning":true}}}`))
		case "EnumerateSessions":
			_, _ = w.Write([]byte(`{"data":{"sessions":[{"sessionName":"Test Session","saveHeaders":[{"saveName":"Test Session_autosave_0","saveDateTime":"2026.08.22-15.38.00"}]}]}}`))
		case "DownloadSaveGame":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="Test Session_autosave_0.sav"`)
			_, _ = w.Write([]byte("FAKE_SAVE_DATA"))
		default:
			http.Error(w, "unknown function", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	ctx := context.Background()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		UPDATE app_settings SET game_api_host = 'localhost', game_api_port = 7777, game_api_token = 'test-token'
		WHERE id = 1`)
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}

	res, err := database.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, status, created_at) VALUES ('dluser', 'x', 'viewer', 'active', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, _ := res.LastInsertId()

	svc := NewService(database)
	// Override client via test hook on service — use internal test helper.
	result, err := svc.downloadLatestWithClient(ctx, userID, ChannelWeb, NewClient(Config{
		Token:   "test-token",
		BaseURL: srv.URL + "/api/v1",
	}))
	if err != nil {
		t.Fatalf("DownloadLatest: %v", err)
	}
	if string(result.Body) != "FAKE_SAVE_DATA" {
		t.Fatalf("body = %q", result.Body)
	}
	if result.Filename != "Test Session_autosave_0.sav" {
		t.Fatalf("filename = %q", result.Filename)
	}

	_, err = svc.downloadLatestWithClient(ctx, userID, ChannelWeb, NewClient(Config{
		Token:   "test-token",
		BaseURL: srv.URL + "/api/v1",
	}))
	if err != ErrRateLimited {
		t.Fatalf("second download err = %v, want ErrRateLimited", err)
	}
}

func TestServiceNotConfigured(t *testing.T) {
	t.Chdir("../..")
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	ctx := context.Background()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	svc := NewService(database)
	_, err = svc.DownloadLatest(ctx, 1, ChannelWeb)
	if err != ErrNotConfigured {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}
