package planner

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"factorymate/internal/auth"
	"factorymate/internal/db"
)

func openPlannerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func seedPlannerUsers(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	svc := auth.NewService(database)
	if _, err := svc.CreateUser(ctx, "owner", "ownerpass", auth.RoleViewer); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if _, err := svc.CreateUser(ctx, "other", "otherpass", auth.RoleViewer); err != nil {
		t.Fatalf("create other: %v", err)
	}
	if _, err := svc.CreateUser(ctx, "admin", "adminpass", auth.RoleAdmin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
}

func insertTestPlan(t *testing.T, ctx context.Context, database *sql.DB, ownerID int64, visibility, status string) int64 {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	graph, _ := json.Marshal(EmptyPlanGraph())
	res, err := database.ExecContext(ctx, `
		INSERT INTO factory_plans (
			owner_user_id, name, visibility, status, solver_options_json, graph_json,
			created_at, updated_at
		) VALUES (?, 'Test Plan', ?, ?, '{}', ?, ?, ?)`,
		ownerID, visibility, status, string(graph), now, now,
	)
	if err != nil {
		t.Fatalf("insert plan: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestLockAcquireAndConflict(t *testing.T) {
	ctx := context.Background()
	database := openPlannerTestDB(t)
	defer database.Close()

	seedPlannerUsers(t, ctx, database)
	planID := insertTestPlan(t, ctx, database, 1, "private", "planning")

	if err := AcquireLock(ctx, database, planID, 1); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := AcquireLock(ctx, database, planID, 2); err == nil {
		t.Fatal("expected lock conflict")
	} else if err != ErrLockHeld {
		t.Fatalf("acquire conflict = %v, want ErrLockHeld", err)
	}

	if err := ReleaseLock(ctx, database, planID, 1); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := AcquireLock(ctx, database, planID, 2); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
}

func TestLockExpirySteal(t *testing.T) {
	ctx := context.Background()
	database := openPlannerTestDB(t)
	defer database.Close()
	seedPlannerUsers(t, ctx, database)
	planID := insertTestPlan(t, ctx, database, 1, "private", "planning")

	past := "2000-01-01T00:00:00Z"
	if _, err := database.ExecContext(ctx, `
		UPDATE factory_plans SET locked_by_user_id = 1, lock_expires_at = ? WHERE id = ?`,
		past, planID,
	); err != nil {
		t.Fatalf("seed expired lock: %v", err)
	}

	if err := AcquireLock(ctx, database, planID, 2); err != nil {
		t.Fatalf("steal expired lock: %v", err)
	}
	lock, err := LoadLockState(ctx, database, planID, 2)
	if err != nil || !lock.Mine {
		t.Fatalf("lock state = %+v err=%v", lock, err)
	}
}

func TestLockArchivedRejected(t *testing.T) {
	ctx := context.Background()
	database := openPlannerTestDB(t)
	defer database.Close()
	seedPlannerUsers(t, ctx, database)
	planID := insertTestPlan(t, ctx, database, 1, "private", "archived")

	if err := AcquireLock(ctx, database, planID, 1); err != ErrPlanArchived {
		t.Fatalf("archived acquire = %v, want ErrPlanArchived", err)
	}
}

func TestForceReleaseByAdmin(t *testing.T) {
	ctx := context.Background()
	database := openPlannerTestDB(t)
	defer database.Close()
	seedPlannerUsers(t, ctx, database)
	planID := insertTestPlan(t, ctx, database, 1, "shared", "planning")

	if err := AcquireLock(ctx, database, planID, 1); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := ForceReleaseLock(ctx, database, planID, 3, true); err != nil {
		t.Fatalf("force release: %v", err)
	}
	lock, err := LoadLockState(ctx, database, planID, 3)
	if err != nil || lock.Held {
		t.Fatalf("lock after force release = %+v err=%v", lock, err)
	}
}
