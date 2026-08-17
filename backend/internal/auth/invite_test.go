package auth_test

import (
	"context"
	"testing"
	"time"

	"factorymate/internal/auth"
	"factorymate/internal/db"
)

func TestInviteLifecycle(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	svc := auth.NewService(database)
	admin, err := svc.CreateUser(ctx, "admin", "secret123", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	inv, err := svc.CreateInvite(ctx, admin.ID, auth.RoleViewer, 24*time.Hour)
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	if inv.Status != auth.InviteStatusPending {
		t.Fatalf("status = %q, want pending", inv.Status)
	}

	loaded, err := svc.GetInviteByToken(ctx, inv.Token)
	if err != nil {
		t.Fatalf("get invite: %v", err)
	}
	if loaded.Role != auth.RoleViewer {
		t.Fatalf("role = %q", loaded.Role)
	}

	user, err := svc.AcceptInvite(ctx, inv.Token, "bob", "bobpass12")
	if err != nil {
		t.Fatalf("accept invite: %v", err)
	}
	if user.Username != "bob" || user.Role != auth.RoleViewer {
		t.Fatalf("user = %+v", user)
	}

	after, err := svc.GetInviteByToken(ctx, inv.Token)
	if err != nil {
		t.Fatalf("get accepted invite: %v", err)
	}
	if after.Status != auth.InviteStatusAccepted {
		t.Fatalf("status = %q, want accepted", after.Status)
	}

	_, err = svc.AcceptInvite(ctx, inv.Token, "other", "otherpass")
	if err == nil {
		t.Fatal("expected second accept to fail")
	}
}

func TestInviteRevokeAndExpire(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	svc := auth.NewService(database)
	admin, err := svc.CreateUser(ctx, "admin", "secret123", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	inv, err := svc.CreateInvite(ctx, admin.ID, auth.RoleViewer, time.Hour)
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	if err := svc.RevokeInvite(ctx, inv.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	revoked, err := svc.GetInviteByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("get invite: %v", err)
	}
	if revoked.Status != auth.InviteStatusRevoked {
		t.Fatalf("status = %q, want revoked", revoked.Status)
	}

	short, err := svc.CreateInvite(ctx, admin.ID, auth.RoleViewer, time.Millisecond)
	if err != nil {
		t.Fatalf("create short invite: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	expired, err := svc.GetInviteByToken(ctx, short.Token)
	if err != nil {
		t.Fatalf("get expired invite: %v", err)
	}
	if expired.Status != auth.InviteStatusExpired {
		t.Fatalf("status = %q, want expired", expired.Status)
	}
}

func TestLastAdminGuard(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	svc := auth.NewService(database)
	admin, err := svc.CreateUser(ctx, "admin", "secret123", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	if err := svc.UpdateUserRole(ctx, admin.ID, auth.RoleViewer); err == nil {
		t.Fatal("expected demote last admin to fail")
	}
	if err := svc.DeleteUser(ctx, admin.ID); err == nil {
		t.Fatal("expected delete last admin to fail")
	}
}
