package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/db"
	"factorymate/internal/notifications"
	"factorymate/internal/notify"
	"factorymate/internal/registration"
)

func TestAccountNotificationsCatalogAndPartialPut(t *testing.T) {
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

	setupAdmin(t, router)
	adminCookie := loginCookie(t, router, "admin", "secret123")

	getResp := getWithCookie(t, router, "/api/account/notifications", adminCookie)
	if getResp.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", getResp.Code, getResp.Body.String())
	}
	var got notifications.UserPrefs
	decodeJSONRecorder(t, getResp, &got)
	if got.Types == nil || got.Catalog == nil {
		t.Fatalf("expected types and catalog, got %+v", got)
	}
	var sawFuse, sawConnection bool
	for _, entry := range got.Catalog {
		if entry.Key == "fuse_tripped" {
			sawFuse = true
			if entry.Label == "" || entry.Category != "power" {
				t.Fatalf("fuse catalog = %+v", entry)
			}
		}
		if entry.Key == "connection_details" || entry.Key == "connection_details_changed" {
			sawConnection = true
		}
	}
	if !sawFuse {
		t.Fatal("catalog missing fuse_tripped")
	}
	if sawConnection {
		t.Fatal("catalog must exclude connection types")
	}

	putResp := putJSONWithCookie(t, router, "/api/account/notifications", adminCookie, map[string]any{
		"types": map[string]bool{"fuse_tripped": true},
	})
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d", putResp.StatusCode)
	}
	var updated notifications.UserPrefs
	if err := json.NewDecoder(putResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode put: %v", err)
	}
	if !updated.Types["fuse_tripped"] {
		t.Fatal("expected fuse_tripped enabled")
	}

	defaultsResp := putJSONWithCookie(t, router, "/api/settings/notification-defaults", adminCookie, map[string]any{
		"types": map[string]bool{"player_joined": true},
		"dmPlayerPersonalDefault": true,
	})
	if defaultsResp.StatusCode != http.StatusOK {
		t.Fatalf("defaults PUT status = %d", defaultsResp.StatusCode)
	}
	var defaults notifications.AdminDefaults
	if err := json.NewDecoder(defaultsResp.Body).Decode(&defaults); err != nil {
		t.Fatalf("decode defaults: %v", err)
	}
	if !defaults.Types["player_joined"] || !defaults.DMPlayerPersonalDefault {
		t.Fatalf("defaults = %+v", defaults)
	}
	if len(defaults.Catalog) == 0 {
		t.Fatal("admin defaults must include catalog")
	}
}
