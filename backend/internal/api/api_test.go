package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"factorymate/internal/api"
	"factorymate/internal/auth"
	"factorymate/internal/db"
)

func TestHealthz(t *testing.T) {
	t.Chdir("../..")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	var body map[string]string
	decodeJSONRecorder(t, resp, &body)
	if body["status"] != "ok" {
		t.Fatalf("body = %v, want status ok", body)
	}
}

func TestReadEndpoints(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	seedAPIFixtures(t, ctx, database)

	svc := auth.NewService(database)
	handler := api.NewHandler(database, svc)
	router := newTestRouter(handler, svc)

	setupAdmin(t, router)
	adminCookie := loginCookie(t, router, "admin", "secret")

	t.Run("GET /api/status", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/status", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
		}
		var body map[string]json.RawMessage
		decodeJSONRecorder(t, resp, &body)

		var serverOnline bool
		if err := json.Unmarshal(body["serverOnline"], &serverOnline); err != nil || !serverOnline {
			t.Fatalf("serverOnline = %v, want true", serverOnline)
		}
		var serverName string
		if err := json.Unmarshal(body["serverName"], &serverName); err != nil || serverName != "GuggiRaid Factory" {
			t.Fatalf("serverName = %q, want GuggiRaid Factory", serverName)
		}
		var onlineCount int
		if err := json.Unmarshal(body["onlinePlayerCount"], &onlineCount); err != nil || onlineCount != 1 {
			t.Fatalf("onlinePlayerCount = %d, want 1", onlineCount)
		}
		var tripped []int
		if err := json.Unmarshal(body["trippedCircuits"], &tripped); err != nil || len(tripped) != 1 || tripped[0] != 1 {
			t.Fatalf("trippedCircuits = %v, want [1]", tripped)
		}

		var milestone struct {
			Name       string `json:"name"`
			TechTier   int    `json:"techTier"`
			UnlockedAt string `json:"unlockedAt"`
		}
		if err := json.Unmarshal(body["latestMilestone"], &milestone); err != nil {
			t.Fatalf("latestMilestone decode: %v", err)
		}
		if milestone.Name != "Oil Processing" || milestone.TechTier != 5 || milestone.UnlockedAt != "2026-08-16T14:30:00Z" {
			t.Fatalf("latestMilestone = %+v", milestone)
		}

		var elevator struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body["elevator"], &elevator); err != nil {
			t.Fatalf("elevator decode: %v", err)
		}
		if elevator.Name != "Space Elevator" {
			t.Fatalf("elevator.name = %q", elevator.Name)
		}
	})

	t.Run("GET /api/players", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/players", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Players []struct {
				ID         string  `json:"id"`
				Name       string  `json:"name"`
				Online     bool    `json:"online"`
				LastSeenAt *string `json:"lastSeenAt"`
			} `json:"players"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Players) != 1 || body.Players[0].Name != "Guggi" || !body.Players[0].Online {
			t.Fatalf("players = %+v", body.Players)
		}
	})

	t.Run("GET /api/players/history", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/players/history", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Items []struct {
				EventType string `json:"eventType"`
			} `json:"items"`
			Total int `json:"total"`
		}
		decodeJSONRecorder(t, resp, &body)
		if body.Total != 1 || body.Items[0].EventType != "joined" {
			t.Fatalf("history = %+v", body)
		}
	})

	t.Run("GET /api/power", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/power", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Circuits []struct {
				CircuitID int  `json:"circuitId"`
				Tripped   bool `json:"tripped"`
			} `json:"circuits"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Circuits) != 1 || body.Circuits[0].CircuitID != 1 || !body.Circuits[0].Tripped {
			t.Fatalf("circuits = %+v", body.Circuits)
		}
	})

	t.Run("GET /api/power/history", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/power/history", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Items []struct {
				EventType string `json:"eventType"`
			} `json:"items"`
			Total int `json:"total"`
		}
		decodeJSONRecorder(t, resp, &body)
		if body.Total != 1 || body.Items[0].EventType != "fuse_tripped" {
			t.Fatalf("history = %+v", body)
		}
	})

	t.Run("GET /api/elevator", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/elevator", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Name         string `json:"name"`
			PhaseNumber  int    `json:"phaseNumber"`
			UpgradeReady bool   `json:"upgradeReady"`
			CurrentPhase []struct {
				Name string `json:"name"`
			} `json:"currentPhase"`
		}
		decodeJSONRecorder(t, resp, &body)
		if body.Name != "Space Elevator" || body.PhaseNumber != 2 || body.UpgradeReady {
			t.Fatalf("elevator = %+v", body)
		}
		if len(body.CurrentPhase) != 1 || body.CurrentPhase[0].Name != "Smart Plating" {
			t.Fatalf("currentPhase = %+v", body.CurrentPhase)
		}
	})

	t.Run("GET /api/milestones", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/milestones", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Groups []struct {
				Type       string `json:"type"`
				Schematics []struct {
					Recipes []struct {
						Name string `json:"name"`
					} `json:"recipes"`
				} `json:"schematics"`
			} `json:"groups"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Groups) != 1 || body.Groups[0].Type != "Milestone" {
			t.Fatalf("groups = %+v", body.Groups)
		}
		if body.Groups[0].Schematics[0].Recipes[0].Name != "Plastic" {
			t.Fatalf("recipes = %+v", body.Groups[0].Schematics[0].Recipes)
		}
	})

	t.Run("GET /api/research", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/research", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Trees []struct {
				Nodes []struct {
					State string `json:"state"`
					Cost  []struct {
						Amount int `json:"amount"`
					} `json:"cost"`
				} `json:"nodes"`
			} `json:"trees"`
		}
		decodeJSONRecorder(t, resp, &body)
		if body.Trees[0].Nodes[0].State != "Purchased" || body.Trees[0].Nodes[0].Cost[0].Amount != 100 {
			t.Fatalf("trees = %+v", body.Trees)
		}
	})

	t.Run("GET /api/production/machines", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/production/machines", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Machines []struct {
				BuildingType string `json:"buildingType"`
				IsProducing  bool   `json:"isProducing"`
			} `json:"machines"`
		}
		decodeJSONRecorder(t, resp, &body)
		if body.Machines[0].BuildingType != "Assembler" || !body.Machines[0].IsProducing {
			t.Fatalf("machines = %+v", body.Machines)
		}
	})

	t.Run("viewer can access session routes", func(t *testing.T) {
		if _, err := svc.CreateUser(ctx, "viewer", "viewerpass", auth.RoleViewer); err != nil {
			t.Fatalf("create viewer: %v", err)
		}
		viewerCookie := loginCookie(t, router, "viewer", "viewerpass")
		resp := getWithCookie(t, router, "/api/players", viewerCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("viewer players status = %d", resp.Code)
		}
	})

	t.Run("viewer forbidden on admin routes", func(t *testing.T) {
		viewerCookie := loginCookie(t, router, "viewer", "viewerpass")
		resp := getWithCookie(t, router, "/api/settings", viewerCookie)
		if resp.Code != http.StatusForbidden {
			t.Fatalf("viewer settings status = %d, want 403", resp.Code)
		}
	})
}

func TestAdminEndpoints(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	svc := auth.NewService(database)
	handler := api.NewHandler(database, svc)
	router := newTestRouter(handler, svc)
	adminCookie := setupAdmin(t, router)

	t.Run("settings get and put", func(t *testing.T) {
		getResp := getWithCookie(t, router, "/api/settings", adminCookie)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get settings status = %d", getResp.Code)
		}
		var settings map[string]any
		decodeJSONRecorder(t, getResp, &settings)
		if settings["serverName"] != "Satisfactory Server" {
			t.Fatalf("serverName = %v", settings["serverName"])
		}

		putReq := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(mustJSON(t, map[string]any{
			"serverName": "GuggiRaid Factory",
			"frmHost":    "192.168.178.42",
			"frmPort":    8889,
		})))
		putReq.Header.Set("Content-Type", "application/json")
		putReq.AddCookie(adminCookie)
		putResp := httptest.NewRecorder()
		router.ServeHTTP(putResp, putReq)
		if putResp.Code != http.StatusOK {
			t.Fatalf("put settings status = %d body=%s", putResp.Code, putResp.Body.String())
		}
	})

	t.Run("notification target CRUD and test send", func(t *testing.T) {
		var gotWebhook bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotWebhook = true
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		createResp := postJSONWithCookie(t, router, "/api/notification-targets", adminCookie, map[string]any{
			"name":         "Main Discord",
			"providerType": "discord",
			"config": map[string]string{
				"webhook_url": srv.URL,
			},
			"enabled": true,
		})
		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("create target status = %d body=%s", createResp.StatusCode, readBody(t, createResp))
		}

		testReq := httptest.NewRequest(http.MethodPost, "/api/notification-targets/1/test", nil)
		testReq.AddCookie(adminCookie)
		testResp := httptest.NewRecorder()
		router.ServeHTTP(testResp, testReq)
		if testResp.Code != http.StatusNoContent {
			t.Fatalf("test send status = %d body=%s", testResp.Code, testResp.Body.String())
		}
		if !gotWebhook {
			t.Fatal("expected webhook to be called")
		}

		delReq := httptest.NewRequest(http.MethodDelete, "/api/notification-targets/1", nil)
		delReq.AddCookie(adminCookie)
		delResp := httptest.NewRecorder()
		router.ServeHTTP(delResp, delReq)
		if delResp.Code != http.StatusNoContent {
			t.Fatalf("delete status = %d", delResp.Code)
		}
	})

	t.Run("message type template preview and enabled", func(t *testing.T) {
		previewResp := postJSONWithCookie(t, router, "/api/message-types/player_joined/template/preview", adminCookie, map[string]any{
			"variant": "embed",
			"template": map[string]any{
				"plain": "hello {PlayerName}",
			},
		})
		if previewResp.StatusCode != http.StatusOK {
			t.Fatalf("preview status = %d body=%s", previewResp.StatusCode, readBody(t, previewResp))
		}

		enabledResp := putJSONWithCookie(t, router, "/api/message-types/player_joined/enabled", adminCookie, map[string]bool{
			"enabled": false,
		})
		if enabledResp.StatusCode != http.StatusOK {
			t.Fatalf("enabled status = %d", enabledResp.StatusCode)
		}
	})

	t.Run("users CRUD", func(t *testing.T) {
		createResp := postJSONWithCookie(t, router, "/api/users", adminCookie, map[string]string{
			"username": "bob",
			"password": "bobpass",
			"role":     "viewer",
		})
		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("create user status = %d", createResp.StatusCode)
		}

		putResp := putJSONWithCookie(t, router, "/api/users/2", adminCookie, map[string]string{
			"role": "admin",
		})
		if putResp.StatusCode != http.StatusOK {
			t.Fatalf("update user status = %d", putResp.StatusCode)
		}

		delReq := httptest.NewRequest(http.MethodDelete, "/api/users/2", nil)
		delReq.AddCookie(adminCookie)
		delResp := httptest.NewRecorder()
		router.ServeHTTP(delResp, delReq)
		if delResp.Code != http.StatusNoContent {
			t.Fatalf("delete user status = %d", delResp.Code)
		}
	})
}

func setupAdmin(t *testing.T, router http.Handler) *http.Cookie {
	resp := postJSON(t, router, "/api/auth/setup", map[string]string{
		"username": "admin",
		"password": "secret",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup status = %d", resp.StatusCode)
	}
	return sessionCookie(resp)
}

func loginCookie(t *testing.T, router http.Handler, username, password string) *http.Cookie {
	resp := postJSON(t, router, "/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login %s status = %d", username, resp.StatusCode)
	}
	return sessionCookie(resp)
}

func getWithCookie(t *testing.T, router http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(cookie)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func postJSONWithCookie(t *testing.T, router http.Handler, path string, cookie *http.Cookie, body any) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(mustJSON(t, body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp.Result()
}

func putJSONWithCookie(t *testing.T, router http.Handler, path string, cookie *http.Cookie, body any) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(mustJSON(t, body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp.Result()
}

func seedAPIFixtures(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)

	_, err := database.ExecContext(ctx, `
		UPDATE app_settings SET server_name = 'GuggiRaid Factory' WHERE id = 1`)
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO server_state (id, server_online, updated_at) VALUES (1, 1, ?)`, now)
	if err != nil {
		t.Fatalf("seed server_state: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO player_state (player_id, name, online, last_seen_at)
		VALUES ('p1', 'Guggi', 1, NULL)`)
	if err != nil {
		t.Fatalf("seed player: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO player_session_events (player_id, player_name, event_type, online_count, occurred_at)
		VALUES ('p1', 'Guggi', 'joined', 3, ?)`, now)
	if err != nil {
		t.Fatalf("seed player event: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO circuit_state (
			circuit_id, tripped, power_capacity, power_production, power_consumed,
			power_max_consumed, battery_differential, battery_percent, battery_capacity,
			battery_time_empty, battery_time_full, updated_at
		) VALUES (1, 1, 1000, 500, 400, 800, 0, 50, 100, '', '', ?)`, now)
	if err != nil {
		t.Fatalf("seed circuit: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO power_circuit_events (circuit_id, event_type, occurred_at)
		VALUES (1, 'fuse_tripped', ?)`, now)
	if err != nil {
		t.Fatalf("seed power event: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO schematic_state (
			schematic_id, name, type, purchased, locked, tech_tier, recipes_json, purchased_at, updated_at
		) VALUES ('s1', 'Oil Processing', 'Milestone', 1, 0, 5,
			'[{"Name":"Plastic","ClassName":"Desc_Plastic_C"}]', '2026-08-16T14:30:00Z', ?)`, now)
	if err != nil {
		t.Fatalf("seed schematic: %v", err)
	}

	phaseJSON := `[{"Name":"Smart Plating","ClassName":"Desc_SpaceElevatorPart_1_C","Amount":10,"RemainingCost":100,"TotalCost":500}]`
	_, err = database.ExecContext(ctx, `
		INSERT INTO elevator_state (elevator_id, name, upgrade_ready, phase_number, current_phase_json, updated_at)
		VALUES ('e1', 'Space Elevator', 0, 2, ?, ?)`, phaseJSON, now)
	if err != nil {
		t.Fatalf("seed elevator: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO research_node_state (node_id, tree_name, name, category, state, tech_tier, cost_json, updated_at)
		VALUES ('n1', 'MAM', 'Oil Processing', 'Oil', 'Purchased', 5, '[{"Name":"Iron Plate","Amount":100}]', ?)`, now)
	if err != nil {
		t.Fatalf("seed research: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO factory_machine_state (
			machine_id, building_type, recipe, manu_speed, is_configured, is_producing, is_paused,
			power_consumed, max_power_consumed, circuit_group_id, ingredients_json, production_json, updated_at
		) VALUES ('m1', 'Assembler', 'Plastic', 100, 1, 1, 0, 10, 15, 1, '[]', '[]', ?)`, now)
	if err != nil {
		t.Fatalf("seed machine: %v", err)
	}
}
