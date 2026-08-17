package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"factorymate/internal/api"
	"factorymate/internal/auth"
	"factorymate/internal/db"

	"github.com/go-chi/chi/v5"
)

func TestAuthFlow(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	svc := auth.NewService(database)
	handler := api.NewHandler(svc)
	router := newTestRouter(handler, svc)

	t.Run("setup once", func(t *testing.T) {
		resp := postJSON(t, router, "/api/auth/setup", map[string]string{
			"username": "admin",
			"password": "secret123",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("setup status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, readBody(t, resp))
		}
		var user auth.User
		decodeJSON(t, resp, &user)
		if user.Username != "admin" || user.Role != auth.RoleAdmin {
			t.Fatalf("user = %+v, want admin/admin", user)
		}
		if c := sessionCookie(resp); c == nil || c.Value == "" {
			t.Fatal("expected session cookie on setup")
		}
	})

	t.Run("second setup rejected", func(t *testing.T) {
		resp := postJSON(t, router, "/api/auth/setup", map[string]string{
			"username": "other",
			"password": "secret123",
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("second setup status = %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("login logout me", func(t *testing.T) {
		loginResp := postJSON(t, router, "/api/auth/login", map[string]string{
			"username": "admin",
			"password": "secret123",
		})
		if loginResp.StatusCode != http.StatusOK {
			t.Fatalf("login status = %d, want %d body=%s", loginResp.StatusCode, http.StatusOK, readBody(t, loginResp))
		}
		cookie := sessionCookie(loginResp)
		if cookie == nil {
			t.Fatal("expected session cookie on login")
		}

		meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		meReq.AddCookie(cookie)
		meResp := httptest.NewRecorder()
		router.ServeHTTP(meResp, meReq)
		if meResp.Code != http.StatusOK {
			t.Fatalf("me status = %d, want %d body=%s", meResp.Code, http.StatusOK, meResp.Body.String())
		}
		var me auth.User
		decodeJSONRecorder(t, meResp, &me)
		if me.Username != "admin" {
			t.Fatalf("me username = %q, want admin", me.Username)
		}

		logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		logoutReq.AddCookie(cookie)
		logoutResp := httptest.NewRecorder()
		router.ServeHTTP(logoutResp, logoutReq)
		if logoutResp.Code != http.StatusNoContent {
			t.Fatalf("logout status = %d, want %d", logoutResp.Code, http.StatusNoContent)
		}

		meAfterLogout := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		meAfterLogout.AddCookie(cookie)
		meAfterLogoutResp := httptest.NewRecorder()
		router.ServeHTTP(meAfterLogoutResp, meAfterLogout)
		if meAfterLogoutResp.Code != http.StatusUnauthorized {
			t.Fatalf("me after logout status = %d, want %d", meAfterLogoutResp.Code, http.StatusUnauthorized)
		}
	})

	t.Run("change password", func(t *testing.T) {
		loginResp := postJSON(t, router, "/api/auth/login", map[string]string{
			"username": "admin",
			"password": "secret123",
		})
		cookie := sessionCookie(loginResp)

		changeReq := httptest.NewRequest(http.MethodPut, "/api/account/password", bytes.NewReader(mustJSON(t, map[string]string{
			"password": "newsecret",
		})))
		changeReq.Header.Set("Content-Type", "application/json")
		changeReq.AddCookie(cookie)
		changeResp := httptest.NewRecorder()
		router.ServeHTTP(changeResp, changeReq)
		if changeResp.Code != http.StatusNoContent {
			t.Fatalf("change password status = %d, want %d body=%s", changeResp.Code, http.StatusNoContent, changeResp.Body.String())
		}

		oldLogin := postJSON(t, router, "/api/auth/login", map[string]string{
			"username": "admin",
			"password": "secret123",
		})
		if oldLogin.StatusCode != http.StatusUnauthorized {
			t.Fatalf("old password login status = %d, want %d", oldLogin.StatusCode, http.StatusUnauthorized)
		}

		newLogin := postJSON(t, router, "/api/auth/login", map[string]string{
			"username": "admin",
			"password": "newsecret",
		})
		if newLogin.StatusCode != http.StatusOK {
			t.Fatalf("new password login status = %d, want %d", newLogin.StatusCode, http.StatusOK)
		}
	})

	t.Run("viewer gets 403 on admin route", func(t *testing.T) {
		if _, err := svc.CreateUser(ctx, "viewer", "viewerpass", auth.RoleViewer); err != nil {
			t.Fatalf("create viewer: %v", err)
		}

		loginResp := postJSON(t, router, "/api/auth/login", map[string]string{
			"username": "viewer",
			"password": "viewerpass",
		})
		if loginResp.StatusCode != http.StatusOK {
			t.Fatalf("viewer login status = %d, want %d", loginResp.StatusCode, http.StatusOK)
		}
		cookie := sessionCookie(loginResp)

		adminReq := httptest.NewRequest(http.MethodGet, "/api/admin-only", nil)
		adminReq.AddCookie(cookie)
		adminResp := httptest.NewRecorder()
		router.ServeHTTP(adminResp, adminReq)
		if adminResp.Code != http.StatusForbidden {
			t.Fatalf("viewer admin route status = %d, want %d body=%s", adminResp.Code, http.StatusForbidden, adminResp.Body.String())
		}
	})

	t.Run("admin route works", func(t *testing.T) {
		loginResp := postJSON(t, router, "/api/auth/login", map[string]string{
			"username": "admin",
			"password": "newsecret",
		})
		cookie := sessionCookie(loginResp)

		adminReq := httptest.NewRequest(http.MethodGet, "/api/admin-only", nil)
		adminReq.AddCookie(cookie)
		adminResp := httptest.NewRecorder()
		router.ServeHTTP(adminResp, adminReq)
		if adminResp.Code != http.StatusOK {
			t.Fatalf("admin route status = %d, want %d body=%s", adminResp.Code, http.StatusOK, adminResp.Body.String())
		}
	})
}

func TestSessionCleanupRemovesExpiredRows(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	svc := auth.NewService(database)
	user, err := svc.CreateUser(ctx, "admin", "secret", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	sess, err := svc.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		UPDATE sessions SET expires_at = '2000-01-01T00:00:00Z' WHERE id = ?`, sess.ID); err != nil {
		t.Fatalf("expire session: %v", err)
	}

	n, err := svc.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if n != 1 {
		t.Fatalf("deleted = %d, want 1", n)
	}

	_, err = svc.GetSession(ctx, sess.ID)
	if err == nil {
		t.Fatal("expected expired session to be gone")
	}
}

func newTestRouter(handler *api.Handler, svc *auth.Service) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Route("/api", func(r chi.Router) {
		handler.Mount(r)
		r.Group(func(r chi.Router) {
			r.Use(svc.RequireSession(apiWriteError))
			r.Use(svc.RequireAdmin(apiWriteError))
			r.Get("/admin-only", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		})
	})
	return r
}

func apiWriteError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database
}

func postJSON(t *testing.T, handler http.Handler, path string, body any) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(mustJSON(t, body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp.Result()
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return b
}

func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	return nil
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

func decodeJSONRecorder(t *testing.T, resp *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}
