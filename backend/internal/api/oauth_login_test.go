package api_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/db"
	"factorymate/internal/notify"
	"factorymate/internal/registration"
)

func TestOAuthLoginDoesNotProvisionUnknownDiscordUser(t *testing.T) {
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
	regSvc := registration.NewService(database, svc)
	handler := newTestHandler(database, svc, regSvc, notify.NewMockDiscordSession())
	router := newTestRouter(handler, svc)

	handler.SetOAuthCodeExchange(func(context.Context, string) (auth.DiscordUserResponse, error) {
		return auth.DiscordUserResponse{ID: "discord-unknown", Username: "ghost"}, nil
	})

	before := userCount(t, ctx, database)

	t.Run("GET login start does not insert a user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/discord", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf("GET /api/auth/discord status = %d, want 503 without Discord app id", resp.Code)
		}
		if got := userCount(t, ctx, database); got != before {
			t.Fatalf("GET /api/auth/discord inserted users: before=%d after=%d", before, got)
		}
	})

	state, err := svc.CreateOAuthState(ctx, auth.OAuthPurposeLogin, auth.OAuthStateMeta{})
	if err != nil {
		t.Fatalf("create login state: %v", err)
	}

	unknown, err := svc.GetUserByExternal(ctx, registration.PlatformDiscord, "discord-unknown")
	if err != nil {
		t.Fatalf("GetUserByExternal: %v", err)
	}
	if unknown != nil {
		t.Fatal("expected GetUserByExternal nil before login callback")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/discord/callback?state="+state+"&code=fake-code", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusFound {
		t.Fatalf("login callback status = %d body=%s", resp.Code, resp.Body.String())
	}
	loc := resp.Header().Get("Location")
	if loc != "https://factorymate.example.com/login?error=not_registered" {
		t.Fatalf("login callback location = %q", loc)
	}
	if cookie := sessionCookie(resp.Result()); cookie != nil && cookie.Value != "" {
		t.Fatal("login callback must not set a session for unknown Discord users")
	}
	if got := userCount(t, ctx, database); got != before {
		t.Fatalf("oauthCallbackLogin inserted a user: before=%d after=%d", before, got)
	}
}

func TestOAuthLoginSetsSessionForExistingExternalUser(t *testing.T) {
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
	regSvc := registration.NewService(database, svc)
	if _, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "linked-user",
		Password:          "",
		PendingPlayerName: "LinkedPlayer",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "discord-known",
		},
		Role:         auth.RoleViewer,
		ForceApprove: true,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	handler := newTestHandler(database, svc, regSvc, notify.NewMockDiscordSession())
	router := newTestRouter(handler, svc)
	handler.SetOAuthCodeExchange(func(context.Context, string) (auth.DiscordUserResponse, error) {
		return auth.DiscordUserResponse{ID: "discord-known", Username: "linked"}, nil
	})

	before := userCount(t, ctx, database)
	state, err := svc.CreateOAuthState(ctx, auth.OAuthPurposeLogin, auth.OAuthStateMeta{})
	if err != nil {
		t.Fatalf("create login state: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/discord/callback?state="+state+"&code=fake-code", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusFound {
		t.Fatalf("login callback status = %d body=%s", resp.Code, resp.Body.String())
	}
	if loc := resp.Header().Get("Location"); loc != "https://factorymate.example.com/" {
		t.Fatalf("login callback location = %q", loc)
	}
	if cookie := sessionCookie(resp.Result()); cookie == nil || cookie.Value == "" {
		t.Fatal("expected session cookie for linked Discord login")
	}
	if got := userCount(t, ctx, database); got != before {
		t.Fatalf("login provisioned extra users: before=%d after=%d", before, got)
	}
}

func userCount(t *testing.T, ctx context.Context, database *sql.DB) int {
	t.Helper()
	var n int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}
