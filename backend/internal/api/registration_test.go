package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/db"
	"factorymate/internal/notify"
	"factorymate/internal/registration"
)

func TestRegistrationAPI(t *testing.T) {
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
		t.Fatalf("set auto approve: %v", err)
	}

	svc := auth.NewService(database)
	regSvc := registration.NewService(database, svc)
	handler := newTestHandler(database, svc, regSvc, notify.NewMockDiscordSession())
	router := newTestRouter(handler, svc)
	adminCookie := setupAdmin(t, router)

	result, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "queue-user",
		Password:          "password123",
		PendingPlayerName: "QueueUser",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "discord-queue",
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/registrations/pending", nil)
	req.AddCookie(adminCookie)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("pending list status = %d body=%s", resp.Code, resp.Body.String())
	}
	var pendingBody struct {
		Registrations []registration.PendingRegistration `json:"registrations"`
	}
	decodeJSONRecorder(t, resp, &pendingBody)
	if len(pendingBody.Registrations) != 1 || pendingBody.Registrations[0].ID != result.User.ID {
		t.Fatalf("pending = %+v", pendingBody.Registrations)
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/registrations/999/approve", nil)
	approveReq.AddCookie(adminCookie)
	approveResp := httptest.NewRecorder()
	router.ServeHTTP(approveResp, approveReq)
	if approveResp.Code != http.StatusConflict {
		t.Fatalf("approve missing status = %d", approveResp.Code)
	}

	approveReq = httptest.NewRequest(http.MethodPost, "/api/registrations/"+strconv.FormatInt(result.User.ID, 10)+"/approve", nil)
	approveReq.AddCookie(adminCookie)
	approveResp = httptest.NewRecorder()
	router.ServeHTTP(approveResp, approveReq)
	if approveResp.Code != http.StatusOK {
		t.Fatalf("approve status = %d body=%s", approveResp.Code, approveResp.Body.String())
	}
}

func TestUnmappedPlayersEndpoint(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO player_state (player_id, name, online, last_seen_at)
		VALUES ('orphan-1', 'Orphan', 1, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert player: %v", err)
	}

	svc := auth.NewService(database)
	regSvc := registration.NewService(database, svc)
	handler := newTestHandler(database, svc, regSvc, notify.NewMockDiscordSession())
	router := newTestRouter(handler, svc)
	adminCookie := setupAdmin(t, router)

	req := httptest.NewRequest(http.MethodGet, "/api/players/unmapped", nil)
	req.AddCookie(adminCookie)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	var body struct {
		Players []registration.UnmappedPlayer `json:"players"`
	}
	decodeJSONRecorder(t, resp, &body)
	if len(body.Players) != 1 || body.Players[0].PlayerID != "orphan-1" {
		t.Fatalf("players = %+v", body.Players)
	}
}

func TestUpdateUserExternalUnlink(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	svc := auth.NewService(database)
	regSvc := registration.NewService(database, svc)
	handler := newTestHandler(database, svc, regSvc, notify.NewMockDiscordSession())
	router := newTestRouter(handler, svc)
	adminCookie := setupAdmin(t, router)

	user, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "linked",
		Password:          "password123",
		PendingPlayerName: "Linked",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "discord-linked",
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/users/"+strconv.FormatInt(user.User.ID, 10)+"/external", bytes.NewReader(mustJSON(t, map[string]any{
		"unlink": true,
	})))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(adminCookie)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unlink status = %d body=%s", resp.Code, resp.Body.String())
	}
	var updated struct {
		ExternalUserID *string `json:"externalUserId"`
		ExternalLinked *string `json:"externalLinkedAt"`
	}
	decodeJSONRecorder(t, resp, &updated)
	if updated.ExternalUserID != nil || updated.ExternalLinked != nil {
		t.Fatalf("expected external unlink, got %+v", updated)
	}
}

func TestPendingApprovalBlocksLoginAPI(t *testing.T) {
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
		t.Fatalf("set auto approve: %v", err)
	}

	svc := auth.NewService(database)
	regSvc := registration.NewService(database, svc)
	router := newTestRouter(newTestHandler(database, svc, regSvc, notify.NewMockDiscordSession()), svc)

	_, err := regSvc.Register(ctx, registration.RegisterParams{
		Username: "blocked",
		Password: "password123",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "discord-blocked",
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	resp := postJSON(t, router, "/api/auth/login", map[string]string{
		"username": "blocked",
		"password": "password123",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("login status = %d, want 403", resp.StatusCode)
	}
}
