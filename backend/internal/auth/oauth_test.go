package auth_test

import (
	"context"
	"testing"
	"time"

	"factorymate/internal/auth"
	"factorymate/internal/db"
	"factorymate/internal/registration"
)

func TestOAuthStateSingleUse(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	t.Setenv("DISCORD_CLIENT_SECRET", "test-secret")
	t.Setenv("FACTORYMATE_PUBLIC_URL", "https://factorymate.example.com")
	t.Setenv("DISCORD_BOT_TOKEN", "fake.token.for.tests")

	svc := auth.NewService(database)
	token, err := svc.CreateOAuthState(ctx, auth.OAuthPurposeLogin, auth.OAuthStateMeta{})
	if err != nil {
		t.Fatalf("create state: %v", err)
	}

	row, err := svc.ConsumeOAuthState(ctx, token, auth.OAuthPurposeLogin)
	if err != nil {
		t.Fatalf("consume state: %v", err)
	}
	if row.Purpose != auth.OAuthPurposeLogin {
		t.Fatalf("purpose = %q", row.Purpose)
	}

	_, err = svc.ConsumeOAuthState(ctx, token, auth.OAuthPurposeLogin)
	if err == nil {
		t.Fatal("expected second consume to fail")
	}
}

func TestPasswordLoginRejectsNullHash(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	authSvc := auth.NewService(database)
	regSvc := registration.NewService(database, authSvc)

	_, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "discord-only",
		Password:          "",
		PendingPlayerName: "PlayerOne",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "discord-sso-1",
		},
		Role: auth.RoleViewer,
	})
	if err != nil {
		t.Fatalf("register discord-only: %v", err)
	}

	_, err = authSvc.Authenticate(ctx, "discord-only", "anything")
	if err == nil {
		t.Fatal("expected password login to fail for discord-only user")
	}
}

func TestLinkExternalWhileLoggedIn(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	svc := auth.NewService(database)
	user, err := svc.CreateUser(ctx, "admin", "secret123", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	linked, err := svc.LinkExternal(ctx, user.ID, registration.PlatformDiscord, "discord-admin", "administrator", "Admin")
	if err != nil {
		t.Fatalf("link external: %v", err)
	}
	if linked.External.UserID == nil || *linked.External.UserID != "discord-admin" {
		t.Fatalf("external = %+v", linked.External)
	}

	_, err = svc.LinkExternal(ctx, user.ID, registration.PlatformDiscord, "other-discord", "other", "Other")
	if err == nil {
		t.Fatal("expected relink while linked to fail")
	}
}

func TestOAuthStateExpires(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	t.Setenv("DISCORD_CLIENT_SECRET", "test-secret")
	t.Setenv("FACTORYMATE_PUBLIC_URL", "https://factorymate.example.com")

	svc := auth.NewService(database)
	token, err := svc.CreateOAuthState(ctx, auth.OAuthPurposeRegister, auth.OAuthStateMeta{
		ExternalUserID: "123",
	})
	if err != nil {
		t.Fatalf("create state: %v", err)
	}

	expired := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if _, err := database.ExecContext(ctx, `UPDATE oauth_states SET expires_at = ?`, expired); err != nil {
		t.Fatalf("backdate expiry: %v", err)
	}

	_, err = svc.ConsumeOAuthState(ctx, token, auth.OAuthPurposeRegister)
	if err == nil {
		t.Fatal("expected expired state to fail")
	}
}
