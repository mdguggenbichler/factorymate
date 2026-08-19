package notifications_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"factorymate/internal/db"
	"factorymate/internal/notifications"
)

func TestPrefsCatalogExcludesConnectionTypes(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	svc, database := openPrefs(t)
	defer database.Close()

	catalog, err := svc.LoadCatalog(ctx)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("expected catalog entries")
	}
	for _, entry := range catalog {
		if notifications.PrefTypeExcluded(entry.Key) {
			t.Fatalf("catalog includes excluded key %q", entry.Key)
		}
		if entry.ChannelTargets == nil {
			t.Fatalf("channelTargets nil for %q", entry.Key)
		}
	}
}

func TestGetAdminDefaultsExpandsLegacyCategoryJSON(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	svc, database := openPrefs(t)
	defer database.Close()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		notifications.KeyDMDefaultsJSON,
		`{"server":false,"player":false,"power":true,"progression":false,"vehicle":false}`,
	); err != nil {
		t.Fatalf("set defaults: %v", err)
	}

	defaults, err := svc.GetAdminDefaults(ctx)
	if err != nil {
		t.Fatalf("GetAdminDefaults: %v", err)
	}
	if !defaults.Types["fuse_tripped"] || !defaults.Types["power_restored"] {
		t.Fatalf("power types = fuse=%v restored=%v, want true", defaults.Types["fuse_tripped"], defaults.Types["power_restored"])
	}
	if defaults.Types["server_online"] {
		t.Fatal("server_online should stay false")
	}
	if _, ok := defaults.Types["connection_details"]; ok {
		t.Fatal("connection_details must not appear in types")
	}
}

func TestSetUserPrefsPartialAndListByType(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	svc, database := openPrefs(t)
	defer database.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, created_at, status, external_platform, external_user_id)
		VALUES (40, 'typed', 'hash', 'viewer', ?, 'active', 'discord', 'discord-typed')`, now); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	on := true
	prefs, err := svc.SetUserPrefs(ctx, 40, notifications.UserPrefsPatch{
		Types:            map[string]bool{"fuse_tripped": true, "power_restored": false},
		DMPlayerPersonal: &on,
	})
	if err != nil {
		t.Fatalf("SetUserPrefs: %v", err)
	}
	if !prefs.Types["fuse_tripped"] || prefs.Types["power_restored"] {
		t.Fatalf("types = %+v", prefs.Types)
	}
	if !prefs.DMPlayerPersonal {
		t.Fatal("expected personal on")
	}

	off := false
	prefs, err = svc.SetUserPrefs(ctx, 40, notifications.UserPrefsPatch{DMPlayerPersonal: &off})
	if err != nil {
		t.Fatalf("partial personal: %v", err)
	}
	if !prefs.Types["fuse_tripped"] {
		t.Fatal("partial update must keep fuse_tripped")
	}
	if prefs.DMPlayerPersonal {
		t.Fatal("personal should be off")
	}

	recipients, err := svc.ListDMRecipients(ctx, "fuse_tripped")
	if err != nil {
		t.Fatalf("ListDMRecipients fuse: %v", err)
	}
	if len(recipients) != 1 || recipients[0].ExternalUserID != "discord-typed" {
		t.Fatalf("fuse recipients = %+v", recipients)
	}
	restored, err := svc.ListDMRecipients(ctx, "power_restored")
	if err != nil {
		t.Fatalf("ListDMRecipients restored: %v", err)
	}
	if len(restored) != 0 {
		t.Fatalf("power_restored recipients = %+v, want none", restored)
	}

	enabled, total := notifications.CategorySummary(prefs.Types, prefs.Catalog, "power")
	if enabled != 1 || total != 2 {
		t.Fatalf("power summary = %d/%d, want 1/2", enabled, total)
	}
}

func TestSetAdminDefaultsPartial(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	svc, database := openPrefs(t)
	defer database.Close()

	trueVal := true
	if _, err := svc.SetAdminDefaults(ctx, notifications.AdminDefaultsPatch{
		Types:                   map[string]bool{"fuse_tripped": true},
		DMPlayerPersonalDefault: &trueVal,
	}); err != nil {
		t.Fatalf("SetAdminDefaults: %v", err)
	}
	defaults, err := svc.GetAdminDefaults(ctx)
	if err != nil {
		t.Fatalf("GetAdminDefaults: %v", err)
	}
	if !defaults.Types["fuse_tripped"] {
		t.Fatal("fuse_tripped default should be true")
	}
	if defaults.Types["server_online"] {
		t.Fatal("unspecified type should remain false")
	}
	if !defaults.DMPlayerPersonalDefault {
		t.Fatal("personal default should be true")
	}
}

func openPrefs(t *testing.T) (*notifications.Service, *sql.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Init(context.Background(), database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	return notifications.NewService(database), database
}
