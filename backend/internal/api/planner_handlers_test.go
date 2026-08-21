package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/db"
	"factorymate/internal/registration"
)

func TestListPlannerPlansEmptyReturnsArray(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	svc := auth.NewService(database)
	regSvc := registration.NewService(database, svc)
	handler := newTestHandler(database, svc, regSvc, nil)
	router := newTestRouter(handler, svc)

	setupAdmin(t, router)
	adminCookie := loginCookie(t, router, "admin", "secret123")

	resp := getWithCookie(t, router, "/api/planner/plans", adminCookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"plans":[]`) {
		t.Fatalf("expected empty plans array, got %s", body)
	}
}

func TestPlannerPlanAPI(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	svc := auth.NewService(database)
	regSvc := registration.NewService(database, svc)
	handler := newTestHandler(database, svc, regSvc, nil)
	router := newTestRouter(handler, svc)

	setupAdmin(t, router)
	adminCookie := loginCookie(t, router, "admin", "secret123")

	if _, err := svc.CreateUser(ctx, "owner", "ownerpass", auth.RoleViewer); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if _, err := svc.CreateUser(ctx, "viewer", "viewerpass", auth.RoleViewer); err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	ownerCookie := loginCookie(t, router, "owner", "ownerpass")
	viewerCookie := loginCookie(t, router, "viewer", "viewerpass")

	createResp := postJSONWithCookie(t, router, "/api/planner/plans", ownerCookie, map[string]any{
		"name":       "Iron plates",
		"visibility": "private",
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create plan status = %d body=%s", createResp.StatusCode, readBody(t, createResp))
	}
	var created struct {
		ID        int64  `json:"id"`
		UpdatedAt string `json:"updatedAt"`
		Status    string `json:"status"`
		CanManage bool   `json:"canManage"`
	}
	decodeJSON(t, createResp, &created)
	if created.Status != "planning" || !created.CanManage {
		t.Fatalf("created = %+v", created)
	}
	planPath := fmt.Sprintf("/api/planner/plans/%d", created.ID)

	t.Run("viewer cannot GET others private plan", func(t *testing.T) {
		resp := getWithCookie(t, router, planPath, viewerCookie)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.Code)
		}
	})

	t.Run("admin lists others private plans", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/planner/plans", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Plans []struct {
				ID         int64  `json:"id"`
				Visibility string `json:"visibility"`
			} `json:"plans"`
		}
		decodeJSONRecorder(t, resp, &body)
		found := false
		for _, p := range body.Plans {
			if p.ID == created.ID && p.Visibility == "private" {
				found = true
			}
		}
		if !found {
			t.Fatalf("admin list missing private plan: %+v", body.Plans)
		}
	})

	lockResp := postJSONWithCookie(t, router, planPath+"/lock", ownerCookie, map[string]any{})
	if lockResp.StatusCode != http.StatusOK {
		t.Fatalf("lock status = %d body=%s", lockResp.StatusCode, readBody(t, lockResp))
	}

	conflictResp := postJSONWithCookie(t, router, planPath+"/lock", adminCookie, map[string]any{})
	if conflictResp.StatusCode != http.StatusConflict {
		t.Fatalf("second lock status = %d, want 409", conflictResp.StatusCode)
	}

	t.Run("graph PUT without lock returns 409", func(t *testing.T) {
		releaseResp := postJSONWithCookie(t, router, planPath+"/lock/release", ownerCookie, map[string]any{})
		releaseResp.Body.Close()
		putResp := putJSONWithCookie(t, router, planPath+"/graph", ownerCookie, map[string]any{
			"updatedAt": created.UpdatedAt,
			"graph": map[string]any{
				"viewport": map[string]any{"x": 0, "y": 0, "zoom": 1},
				"nodes":    []any{},
				"edges":    []any{},
			},
		})
		if putResp.StatusCode != http.StatusConflict {
			t.Fatalf("put without lock status = %d, want 409", putResp.StatusCode)
		}
		putResp.Body.Close()
	})

	lockResp2 := postJSONWithCookie(t, router, planPath+"/lock", ownerCookie, map[string]any{})
	if lockResp2.StatusCode != http.StatusOK {
		t.Fatalf("re-lock status = %d", lockResp2.StatusCode)
	}
	lockResp2.Body.Close()

	suggestResp := postJSONWithCookie(t, router, planPath+"/suggest", ownerCookie, map[string]any{
		"itemClass":  "Desc_IronPlate_C",
		"ratePerMin": 60,
		"apply":      true,
	})
	if suggestResp.StatusCode != http.StatusOK {
		t.Fatalf("suggest status = %d body=%s", suggestResp.StatusCode, readBody(t, suggestResp))
	}
	var suggestBody struct {
		Graph struct {
			Nodes []struct {
				RecipeClass string  `json:"recipeClass"`
				Count       float64 `json:"count"`
			} `json:"nodes"`
		} `json:"graph"`
	}
	decodeJSON(t, suggestResp, &suggestBody)
	if len(suggestBody.Graph.Nodes) < 3 {
		t.Fatalf("suggest nodes = %d, want >= 3", len(suggestBody.Graph.Nodes))
	}
	suggestResp.Body.Close()

	detailResp := getWithCookie(t, router, planPath, ownerCookie)
	var detail struct {
		UpdatedAt   string `json:"updatedAt"`
		HasBaseline bool   `json:"hasBaseline"`
	}
	decodeJSONRecorder(t, detailResp, &detail)
	if !detail.HasBaseline {
		t.Fatal("expected baseline after apply suggest")
	}

	patchResp := patchJSONWithCookie(t, router, planPath, ownerCookie, map[string]any{
		"status": "archived",
	})
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("archive status = %d body=%s", patchResp.StatusCode, readBody(t, patchResp))
	}
	patchResp.Body.Close()

	t.Run("archived plan rejects lock graph suggest", func(t *testing.T) {
		lockArchived := postJSONWithCookie(t, router, planPath+"/lock", ownerCookie, map[string]any{})
		if lockArchived.StatusCode != http.StatusForbidden {
			t.Fatalf("archived lock status = %d, want 403", lockArchived.StatusCode)
		}
		lockArchived.Body.Close()

		putArchived := putJSONWithCookie(t, router, planPath+"/graph", ownerCookie, map[string]any{
			"updatedAt": detail.UpdatedAt,
			"graph":     suggestBody.Graph,
		})
		if putArchived.StatusCode != http.StatusForbidden {
			t.Fatalf("archived graph status = %d, want 403", putArchived.StatusCode)
		}
		putArchived.Body.Close()

		suggestArchived := postJSONWithCookie(t, router, planPath+"/suggest", ownerCookie, map[string]any{
			"itemClass":  "Desc_IronPlate_C",
			"ratePerMin": 60,
		})
		if suggestArchived.StatusCode != http.StatusForbidden {
			t.Fatalf("archived suggest status = %d, want 403", suggestArchived.StatusCode)
		}
		suggestArchived.Body.Close()
	})

	sharedResp := postJSONWithCookie(t, router, "/api/planner/plans", ownerCookie, map[string]any{
		"name":       "Shared plan",
		"visibility": "shared",
	})
	var shared struct{ ID int64 `json:"id"` }
	decodeJSON(t, sharedResp, &shared)
	sharedResp.Body.Close()
	sharedPath := fmt.Sprintf("/api/planner/plans/%d", shared.ID)

	sharedGet := getWithCookie(t, router, sharedPath, viewerCookie)
	if sharedGet.Code != http.StatusOK {
		t.Fatalf("viewer shared get status = %d", sharedGet.Code)
	}

	ownerLock := postJSONWithCookie(t, router, sharedPath+"/lock", ownerCookie, map[string]any{})
	if ownerLock.StatusCode != http.StatusOK {
		t.Fatalf("shared lock status = %d", ownerLock.StatusCode)
	}
	ownerLock.Body.Close()

	forceRelease := postJSONWithCookie(t, router, sharedPath+"/lock/force-release", adminCookie, map[string]any{})
	if forceRelease.StatusCode != http.StatusOK {
		t.Fatalf("force release status = %d body=%s", forceRelease.StatusCode, readBody(t, forceRelease))
	}
	forceRelease.Body.Close()
}

func patchJSONWithCookie(t *testing.T, router http.Handler, path string, cookie *http.Cookie, body any) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(mustJSON(t, body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp.Result()
}
