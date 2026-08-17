package mods_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"factorymate/internal/db"
	"factorymate/internal/frm"
	"factorymate/internal/mods"
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

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "frm", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func TestListFromFRMFixture(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	var rawMods []frm.Mod
	if err := json.Unmarshal(readFixture(t, "getModList.json"), &rawMods); err != nil {
		t.Fatalf("unmarshal mods: %v", err)
	}

	srv := mods.NewModListTestServer(rawMods)
	defer srv.Server.Close()

	_, err := database.ExecContext(ctx, `
		UPDATE app_settings SET frm_host = ?, frm_port = ? WHERE id = 1`,
		srv.Host, srv.Port)
	if err != nil {
		t.Fatalf("update frm settings: %v", err)
	}

	svc := mods.NewService(database, func(ctx context.Context) (*frm.Client, error) {
		return frm.NewClient(frm.Config{Host: srv.Host, Port: srv.Port}), nil
	})

	list, err := svc.Refresh(ctx)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !list.FRMReachable {
		t.Fatal("expected frm reachable")
	}
	if list.GameBuild != "502094.0.0" || list.SMLVersion != "3.12.0" {
		t.Fatalf("build/sml = %s / %s", list.GameBuild, list.SMLVersion)
	}
	if len(list.Mods) != 5 {
		t.Fatalf("mods count = %d, want 5", len(list.Mods))
	}
}

func TestSMMProfileMatchesFixtureStructure(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	var rawMods []frm.Mod
	if err := json.Unmarshal(readFixture(t, "getModList.json"), &rawMods); err != nil {
		t.Fatalf("unmarshal mods: %v", err)
	}

	fixturePath := filepath.Join("..", "docs", "examples", "smm_profiles", "Default-2026-08-17-13-07-01.smmprofile")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read smm fixture: %v", err)
	}
	var expected map[string]any
	if err := json.Unmarshal(fixtureData, &expected); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	srv := mods.NewModListTestServer(rawMods)
	defer srv.Server.Close()
	_, err = database.ExecContext(ctx, `
		UPDATE app_settings SET frm_host = ?, frm_port = ? WHERE id = 1`,
		srv.Host, srv.Port)
	if err != nil {
		t.Fatalf("update frm: %v", err)
	}

	svc := mods.NewService(database, func(ctx context.Context) (*frm.Client, error) {
		return frm.NewClient(frm.Config{Host: srv.Host, Port: srv.Port}), nil
	})
	svc.FicsitClient = mods.MockFicsitFromFixture(fixtureData)

	_, err = svc.Refresh(ctx)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	data, filename, err := svc.GenerateSMMProfile(ctx)
	if err != nil {
		t.Fatalf("generate profile: %v", err)
	}
	if filename != "FactoryMate-Server.smmprofile" {
		t.Fatalf("filename = %q", filename)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal profile: %v", err)
	}

	gotProfile, _ := got["profile"].(map[string]any)
	expProfile, _ := expected["profile"].(map[string]any)
	gotMods, _ := gotProfile["mods"].(map[string]any)
	expMods, _ := expProfile["mods"].(map[string]any)
	if len(gotMods) != len(expMods) {
		t.Fatalf("profile mods count = %d, want %d", len(gotMods), len(expMods))
	}

	gotLock, _ := got["lockfile"].(map[string]any)
	expLock, _ := expected["lockfile"].(map[string]any)
	gotLockMods, _ := gotLock["mods"].(map[string]any)
	expLockMods, _ := expLock["mods"].(map[string]any)
	if len(gotLockMods) != len(expLockMods) {
		t.Fatalf("lockfile mods count = %d, want %d", len(gotLockMods), len(expLockMods))
	}

	gotMeta, _ := got["metadata"].(map[string]any)
	expMeta, _ := expected["metadata"].(map[string]any)
	if gotMeta["gameVersion"] != expMeta["gameVersion"] {
		t.Fatalf("gameVersion = %v, want %v", gotMeta["gameVersion"], expMeta["gameVersion"])
	}

	autoSort, _ := gotLockMods["AutoSort"].(map[string]any)
	targets, _ := autoSort["targets"].(map[string]any)
	win, _ := targets["Windows"].(map[string]any)
	expAuto, _ := expLockMods["AutoSort"].(map[string]any)
	expTargets, _ := expAuto["targets"].(map[string]any)
	expWin, _ := expTargets["Windows"].(map[string]any)
	if win["hash"] != expWin["hash"] {
		t.Fatalf("AutoSort Windows hash = %v, want %v", win["hash"], expWin["hash"])
	}
}
