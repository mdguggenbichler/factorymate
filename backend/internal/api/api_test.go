package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"factorymate/internal/api"
	"factorymate/internal/auth"
	"factorymate/internal/connection"
	"factorymate/internal/db"
	"factorymate/internal/frm"
	"factorymate/internal/mods"
	"factorymate/internal/notify"
	"factorymate/internal/registration"
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
	regSvc := registration.NewService(database, svc)
	handler := newTestHandler(database, svc, regSvc, notify.NewMockDiscordSession())
	router := newTestRouter(handler, svc)

	setupAdmin(t, router)
	adminCookie := loginCookie(t, router, "admin", "secret123")

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
					State        string `json:"state"`
					Coordinates  *struct {
						X int `json:"x"`
						Y int `json:"y"`
					} `json:"coordinates"`
					Parents []struct {
						X int `json:"x"`
						Y int `json:"y"`
					} `json:"parents"`
					Cost []struct {
						Amount int `json:"amount"`
					} `json:"cost"`
				} `json:"nodes"`
			} `json:"trees"`
		}
		decodeJSONRecorder(t, resp, &body)
		node := body.Trees[0].Nodes[0]
		if node.State != "Purchased" || node.Cost[0].Amount != 100 {
			t.Fatalf("trees = %+v", body.Trees)
		}
		if node.Coordinates == nil || node.Coordinates.X != 2 || node.Coordinates.Y != 3 {
			t.Fatalf("coordinates = %+v, want (2,3)", node.Coordinates)
		}
		if len(node.Parents) != 1 || node.Parents[0].X != 1 || node.Parents[0].Y != 0 {
			t.Fatalf("parents = %+v, want [{1,0}]", node.Parents)
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

	t.Run("GET /api/power/metrics", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/power/metrics?circuit=1", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
		}
		var body struct {
			Items []struct {
				CircuitID       int     `json:"circuitId"`
				PowerProduction float64 `json:"powerProduction"`
				PowerConsumed   float64 `json:"powerConsumed"`
				CapturedAt      string  `json:"capturedAt"`
			} `json:"items"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Items) != 1 || body.Items[0].CircuitID != 1 || body.Items[0].PowerProduction != 500 {
			t.Fatalf("items = %+v", body.Items)
		}
	})

	t.Run("GET /api/production", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/production?item=Desc_Plastic_C", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Items []struct {
				ItemClassName   string  `json:"itemClassName"`
				ItemDisplayName string  `json:"itemDisplayName"`
				ProducedPerMin  float64 `json:"producedPerMin"`
				ConsumedPerMin  float64 `json:"consumedPerMin"`
			} `json:"items"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Items) != 1 || body.Items[0].ItemDisplayName != "Plastic" {
			t.Fatalf("items = %+v", body.Items)
		}
	})

	t.Run("GET /api/production/items", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/production/items", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Items []string `json:"items"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Items) != 1 || body.Items[0] != "Desc_Plastic_C" {
			t.Fatalf("items = %+v", body.Items)
		}
	})

	t.Run("GET /api/production/current", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/production/current", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Items []struct {
				ItemDisplayName string  `json:"itemDisplayName"`
				ProdPercent     float64 `json:"prodPercent"`
				TransferType    string  `json:"transferType"`
			} `json:"items"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Items) != 1 || body.Items[0].ItemDisplayName != "Plastic" || body.Items[0].TransferType != "Belt" {
			t.Fatalf("items = %+v", body.Items)
		}
	})

	t.Run("GET /api/resource-sink", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/resource-sink", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			NumCoupon   int     `json:"numCoupon"`
			Percent     float64 `json:"percent"`
			TotalPoints int     `json:"totalPoints"`
		}
		decodeJSONRecorder(t, resp, &body)
		if body.NumCoupon != 5 || body.TotalPoints != 50000 {
			t.Fatalf("body = %+v", body)
		}
	})

	t.Run("GET /api/resource-sink/history", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/resource-sink/history", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Items []struct {
				NumCoupon   int `json:"numCoupon"`
				TotalPoints int `json:"totalPoints"`
			} `json:"items"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Items) != 1 || body.Items[0].NumCoupon != 5 {
			t.Fatalf("items = %+v", body.Items)
		}
	})

	t.Run("GET /api/drones", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/drones", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Drones []struct {
				DroneID           string `json:"droneId"`
				HomeStation       string `json:"homeStation"`
				CurrentFlyingMode string `json:"currentFlyingMode"`
			} `json:"drones"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Drones) != 1 || body.Drones[0].HomeStation != "Home" || body.Drones[0].CurrentFlyingMode != "Flying" {
			t.Fatalf("drones = %+v", body.Drones)
		}
	})

	t.Run("GET /api/doggos", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/doggos", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Doggos []struct {
				DoggoID   string `json:"doggoId"`
				Name      string `json:"name"`
				Inventory []struct {
					Name string `json:"Name"`
				} `json:"inventory"`
			} `json:"doggos"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Doggos) != 1 || body.Doggos[0].Name != "Buddy" || len(body.Doggos[0].Inventory) != 1 {
			t.Fatalf("doggos = %+v", body.Doggos)
		}
	})

	t.Run("GET /api/vehicles", func(t *testing.T) {
		resp := getWithCookie(t, router, "/api/vehicles", adminCookie)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d", resp.Code)
		}
		var body struct {
			Trains []struct {
				TrainID string `json:"trainId"`
				Name    string `json:"name"`
				Derailed bool  `json:"derailed"`
			} `json:"trains"`
			Vehicles []struct {
				VehicleID   string `json:"vehicleId"`
				DisplayName string `json:"displayName"`
				Driver      string `json:"driver"`
			} `json:"vehicles"`
		}
		decodeJSONRecorder(t, resp, &body)
		if len(body.Trains) != 1 || body.Trains[0].Name != "Express" || body.Trains[0].Derailed {
			t.Fatalf("trains = %+v", body.Trains)
		}
		if len(body.Vehicles) != 1 || body.Vehicles[0].DisplayName != "Explorer" || body.Vehicles[0].Driver != "Guggi" {
			t.Fatalf("vehicles = %+v", body.Vehicles)
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
		invitesResp := getWithCookie(t, router, "/api/invites", viewerCookie)
		if invitesResp.Code != http.StatusForbidden {
			t.Fatalf("viewer invites status = %d, want 403", invitesResp.Code)
		}
		createInviteResp := postJSONWithCookie(t, router, "/api/invites", viewerCookie, map[string]string{
			"role": "viewer",
		})
		defer createInviteResp.Body.Close()
		if createInviteResp.StatusCode != http.StatusForbidden {
			t.Fatalf("viewer create invite status = %d, want 403", createInviteResp.StatusCode)
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
	regSvc := registration.NewService(database, svc)
	mockDiscord := notify.NewMockDiscordSession()
	handler := newTestHandler(database, svc, regSvc, mockDiscord)
	router := newTestRouter(handler, svc)
	adminCookie := setupAdmin(t, router)
	seedAdminAPIFixtures(t, ctx, database)

	t.Run("settings get and put", func(t *testing.T) {
		frmSrv := newMockFRMServer(t, map[string][]byte{
			"/getSessionInfo": []byte(`{"SessionName":"GuggiRaid Factory","IsPaused":false}`),
		})
		defer frmSrv.Close()
		host, port := frmServerHostPort(t, frmSrv)

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
			"frmHost": host,
			"frmPort": port,
		})))
		putReq.Header.Set("Content-Type", "application/json")
		putReq.AddCookie(adminCookie)
		putResp := httptest.NewRecorder()
		router.ServeHTTP(putResp, putReq)
		if putResp.Code != http.StatusOK {
			t.Fatalf("put settings status = %d body=%s", putResp.Code, putResp.Body.String())
		}
		var updated map[string]any
		decodeJSONRecorder(t, putResp, &updated)
		if updated["serverName"] != "GuggiRaid Factory" {
			t.Fatalf("serverName = %v", updated["serverName"])
		}

		testResp := postJSONWithCookie(t, router, "/api/settings/frm/test", adminCookie, map[string]any{
			"frmHost": host,
			"frmPort": port,
		})
		defer testResp.Body.Close()
		if testResp.StatusCode != http.StatusOK {
			t.Fatalf("frm test status = %d body=%s", testResp.StatusCode, readBody(t, testResp))
		}
	})

	t.Run("notification target CRUD and test send", func(t *testing.T) {
		createResp := postJSONWithCookie(t, router, "/api/notification-targets", adminCookie, map[string]any{
			"name":         "Main Discord",
			"providerType": "discord",
			"config": map[string]string{
				"channelId": "123456789",
			},
			"enabled": true,
		})
		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("create target status = %d body=%s", createResp.StatusCode, readBody(t, createResp))
		}

		listResp := getWithCookie(t, router, "/api/notification-targets", adminCookie)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list targets status = %d", listResp.Code)
		}
		var listBody struct {
			Targets []struct {
				ID           int64  `json:"id"`
				Name         string `json:"name"`
				ProviderType string `json:"providerType"`
				Enabled      bool   `json:"enabled"`
			} `json:"targets"`
		}
		decodeJSONRecorder(t, listResp, &listBody)
		if len(listBody.Targets) != 1 || listBody.Targets[0].Name != "Main Discord" || listBody.Targets[0].ProviderType != "discord" {
			t.Fatalf("targets = %+v", listBody.Targets)
		}

		putResp := putJSONWithCookie(t, router, "/api/notification-targets/1", adminCookie, map[string]any{
			"name":    "Updated Discord",
			"enabled": false,
		})
		if putResp.StatusCode != http.StatusOK {
			t.Fatalf("update target status = %d body=%s", putResp.StatusCode, readBody(t, putResp))
		}
		var updatedTarget struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		decodeJSON(t, putResp, &updatedTarget)
		if updatedTarget.Name != "Updated Discord" || updatedTarget.Enabled {
			t.Fatalf("updated target = %+v", updatedTarget)
		}

		testReq := httptest.NewRequest(http.MethodPost, "/api/notification-targets/1/test", nil)
		testReq.AddCookie(adminCookie)
		testResp := httptest.NewRecorder()
		router.ServeHTTP(testResp, testReq)
		if testResp.Code != http.StatusNoContent {
			t.Fatalf("test send status = %d body=%s", testResp.Code, testResp.Body.String())
		}
		if len(mockDiscord.ChannelCalls) == 0 {
			t.Fatal("expected discord send to be called")
		}

		delReq := httptest.NewRequest(http.MethodDelete, "/api/notification-targets/1", nil)
		delReq.AddCookie(adminCookie)
		delResp := httptest.NewRecorder()
		router.ServeHTTP(delResp, delReq)
		if delResp.Code != http.StatusNoContent {
			t.Fatalf("delete status = %d", delResp.Code)
		}
	})

	t.Run("notification target threadId clear and trim", func(t *testing.T) {
		createResp := postJSONWithCookie(t, router, "/api/notification-targets", adminCookie, map[string]any{
			"name":         "Thread Target",
			"providerType": "discord",
			"config": map[string]string{
				"channelId": " 123456789 ",
				"threadId":  " 987654321 ",
			},
			"enabled": true,
		})
		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("create target status = %d body=%s", createResp.StatusCode, readBody(t, createResp))
		}
		var created struct {
			ID     int64          `json:"id"`
			Config map[string]any `json:"config"`
		}
		decodeJSON(t, createResp, &created)
		if created.Config["channelId"] != "123456789" {
			t.Fatalf("create channelId = %v, want trimmed", created.Config["channelId"])
		}
		if created.Config["threadId"] != "987654321" {
			t.Fatalf("create threadId = %v, want trimmed", created.Config["threadId"])
		}

		clearResp := putJSONWithCookie(t, router, fmt.Sprintf("/api/notification-targets/%d", created.ID), adminCookie, map[string]any{
			"config": map[string]string{
				"threadId": "",
			},
		})
		if clearResp.StatusCode != http.StatusOK {
			t.Fatalf("clear thread status = %d body=%s", clearResp.StatusCode, readBody(t, clearResp))
		}
		var cleared struct {
			Config map[string]any `json:"config"`
		}
		decodeJSON(t, clearResp, &cleared)
		if _, ok := cleared.Config["threadId"]; ok {
			t.Fatalf("expected threadId omitted after clear, got %+v", cleared.Config)
		}

		updateResp := putJSONWithCookie(t, router, fmt.Sprintf("/api/notification-targets/%d", created.ID), adminCookie, map[string]any{
			"config": map[string]string{
				"channelId": " 111222333 ",
			},
		})
		if updateResp.StatusCode != http.StatusOK {
			t.Fatalf("update channel status = %d body=%s", updateResp.StatusCode, readBody(t, updateResp))
		}
		var updated struct {
			Config map[string]any `json:"config"`
		}
		decodeJSON(t, updateResp, &updated)
		if updated.Config["channelId"] != "111222333" {
			t.Fatalf("updated channelId = %v, want trimmed", updated.Config["channelId"])
		}
	})

	t.Run("message types list template reset targets and validation", func(t *testing.T) {
		createResp := postJSONWithCookie(t, router, "/api/notification-targets", adminCookie, map[string]any{
			"name":         "Alerts",
			"providerType": "discord",
			"config": map[string]string{
				"channelId": "987654321",
			},
			"enabled": true,
		})
		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("create target status = %d", createResp.StatusCode)
		}

		listResp := getWithCookie(t, router, "/api/message-types", adminCookie)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list message types status = %d", listResp.Code)
		}
		var listBody struct {
			MessageTypes []struct {
				Key       string `json:"key"`
				Enabled   bool   `json:"enabled"`
				TargetIds []int  `json:"targetIds"`
			} `json:"messageTypes"`
		}
		decodeJSONRecorder(t, listResp, &listBody)
		if len(listBody.MessageTypes) == 0 {
			t.Fatal("expected seeded message types")
		}
		found := false
		for _, mt := range listBody.MessageTypes {
			if mt.Key == "player_joined" {
				found = true
				if !mt.Enabled {
					t.Fatalf("player_joined enabled = false, want true")
				}
				break
			}
		}
		if !found {
			t.Fatal("player_joined not in message types list")
		}

		templateResp := putJSONWithCookie(t, router, "/api/message-types/player_joined/template", adminCookie, map[string]string{
			"plain": "👋 {PlayerName} joined ({OnlineCount} online)",
		})
		if templateResp.StatusCode != http.StatusOK {
			t.Fatalf("put template status = %d body=%s", templateResp.StatusCode, readBody(t, templateResp))
		}

		invalidResp := putJSONWithCookie(t, router, "/api/message-types/player_joined/template", adminCookie, map[string]string{
			"plain": "Hello {NotARealVariable}",
		})
		if invalidResp.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid template status = %d, want 400 body=%s", invalidResp.StatusCode, readBody(t, invalidResp))
		}
		var errBody struct {
			Error string `json:"error"`
		}
		decodeJSON(t, invalidResp, &errBody)
		if errBody.Error == "" {
			t.Fatalf("expected error message in body")
		}

		resetReq := httptest.NewRequest(http.MethodPost, "/api/message-types/player_joined/template/reset?variant=plain", nil)
		resetReq.AddCookie(adminCookie)
		resetResp := httptest.NewRecorder()
		router.ServeHTTP(resetResp, resetReq)
		if resetResp.Code != http.StatusOK {
			t.Fatalf("reset template status = %d body=%s", resetResp.Code, resetResp.Body.String())
		}

		targetsResp := putJSONWithCookie(t, router, "/api/message-types/player_joined/targets", adminCookie, map[string][]int64{
			"targetIds": {1},
		})
		if targetsResp.StatusCode != http.StatusOK {
			t.Fatalf("put targets status = %d body=%s", targetsResp.StatusCode, readBody(t, targetsResp))
		}
		var targetsBody struct {
			TargetIds []int64 `json:"targetIds"`
		}
		decodeJSON(t, targetsResp, &targetsBody)
		if len(targetsBody.TargetIds) != 1 || targetsBody.TargetIds[0] != 1 {
			t.Fatalf("targetIds = %+v", targetsBody.TargetIds)
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

	t.Run("invites and users", func(t *testing.T) {
		listResp := getWithCookie(t, router, "/api/users", adminCookie)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list users status = %d", listResp.Code)
		}
		var listBody struct {
			Users []struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
				Role     string `json:"role"`
				Status   string `json:"status"`
			} `json:"users"`
		}
		decodeJSONRecorder(t, listResp, &listBody)
		if len(listBody.Users) != 1 || listBody.Users[0].Username != "admin" || listBody.Users[0].Role != "admin" {
			t.Fatalf("users = %+v", listBody.Users)
		}

		createResp := postJSONWithCookie(t, router, "/api/invites", adminCookie, map[string]string{
			"role": "viewer",
		})
		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("create invite status = %d body=%s", createResp.StatusCode, readBody(t, createResp))
		}
		var inviteBody struct {
			Token      string `json:"token"`
			InvitePath string `json:"invitePath"`
		}
		decodeJSON(t, createResp, &inviteBody)
		if inviteBody.Token == "" || inviteBody.InvitePath == "" {
			t.Fatalf("invite = %+v", inviteBody)
		}

		acceptResp := postJSON(t, router, "/api/invites/"+inviteBody.Token+"/accept", map[string]string{
			"username": "bob",
			"password": "bobpass12",
		})
		if acceptResp.StatusCode != http.StatusCreated {
			t.Fatalf("accept invite status = %d body=%s", acceptResp.StatusCode, readBody(t, acceptResp))
		}

		dupInviteResp := postJSONWithCookie(t, router, "/api/invites", adminCookie, map[string]string{
			"role": "viewer",
		})
		if dupInviteResp.StatusCode != http.StatusCreated {
			t.Fatalf("create duplicate invite status = %d", dupInviteResp.StatusCode)
		}
		var dupInviteBody struct {
			Token string `json:"token"`
		}
		decodeJSON(t, dupInviteResp, &dupInviteBody)
		dupAcceptResp := postJSON(t, router, "/api/invites/"+dupInviteBody.Token+"/accept", map[string]string{
			"username": "bob",
			"password": "otherpass",
		})
		if dupAcceptResp.StatusCode != http.StatusConflict {
			t.Fatalf("duplicate username accept status = %d, want 409 body=%s", dupAcceptResp.StatusCode, readBody(t, dupAcceptResp))
		}

		_, err := database.ExecContext(ctx, `
			INSERT INTO player_state (player_id, name, online, last_seen_at)
			VALUES ('player-1', 'BobPlayer', 1, NULL)`)
		if err != nil {
			t.Fatalf("seed player: %v", err)
		}

		putResp := putJSONWithCookie(t, router, "/api/users/2", adminCookie, map[string]any{
			"role":     "admin",
			"playerId": "player-1",
		})
		if putResp.StatusCode != http.StatusOK {
			t.Fatalf("update user status = %d body=%s", putResp.StatusCode, readBody(t, putResp))
		}

		delReq := httptest.NewRequest(http.MethodDelete, "/api/users/2", nil)
		delReq.AddCookie(adminCookie)
		delResp := httptest.NewRecorder()
		router.ServeHTTP(delResp, delReq)
		if delResp.Code != http.StatusNoContent {
			t.Fatalf("delete user status = %d", delResp.Code)
		}
	})

	t.Run("notification log and elevator unknown log", func(t *testing.T) {
		logResp := getWithCookie(t, router, "/api/notification-log", adminCookie)
		if logResp.Code != http.StatusOK {
			t.Fatalf("notification log status = %d", logResp.Code)
		}
		var logBody struct {
			Items []struct {
				MessageTypeKey          string  `json:"messageTypeKey"`
				TargetID                *int64  `json:"targetId"`
				TargetName              *string `json:"targetName"`
				DeliveryMode            string  `json:"deliveryMode"`
				RecipientExternalUserID *string `json:"recipientExternalUserId"`
				RenderedPreview         string  `json:"renderedPreview"`
				Success                 bool    `json:"success"`
			} `json:"items"`
			Total int `json:"total"`
		}
		decodeJSONRecorder(t, logResp, &logBody)
		if logBody.Total != 2 {
			t.Fatalf("notification log total = %d, want 2: %+v", logBody.Total, logBody)
		}
		var playerJoinedOK, targetTestOK bool
		for _, item := range logBody.Items {
			if item.DeliveryMode != "channel" {
				t.Fatalf("deliveryMode = %q, want channel", item.DeliveryMode)
			}
			switch item.MessageTypeKey {
			case "player_joined":
				playerJoinedOK = item.Success
			case "target_test":
				targetTestOK = item.Success
			default:
				t.Fatalf("unexpected message type in log: %q", item.MessageTypeKey)
			}
		}
		if !playerJoinedOK || !targetTestOK {
			t.Fatalf("notification log = %+v", logBody)
		}

		unknownResp := getWithCookie(t, router, "/api/elevator/unknown-log", adminCookie)
		if unknownResp.Code != http.StatusOK {
			t.Fatalf("unknown log status = %d", unknownResp.Code)
		}
		var unknownBody struct {
			Items []struct {
				ID       int64 `json:"id"`
				Resolved bool  `json:"resolved"`
			} `json:"items"`
		}
		decodeJSONRecorder(t, unknownResp, &unknownBody)
		if len(unknownBody.Items) != 1 || unknownBody.Items[0].Resolved {
			t.Fatalf("unknown log = %+v", unknownBody.Items)
		}

		resolveReq := httptest.NewRequest(http.MethodPost, "/api/elevator/unknown-log/1/resolve", nil)
		resolveReq.AddCookie(adminCookie)
		resolveResp := httptest.NewRecorder()
		router.ServeHTTP(resolveResp, resolveReq)
		if resolveResp.Code != http.StatusNoContent {
			t.Fatalf("resolve status = %d body=%s", resolveResp.Code, resolveResp.Body.String())
		}

		afterResp := getWithCookie(t, router, "/api/elevator/unknown-log", adminCookie)
		var afterBody struct {
			Items []struct {
				ID       int64 `json:"id"`
				Resolved bool  `json:"resolved"`
			} `json:"items"`
		}
		decodeJSONRecorder(t, afterResp, &afterBody)
		if len(afterBody.Items) != 1 || !afterBody.Items[0].Resolved {
			t.Fatalf("after resolve = %+v", afterBody.Items)
		}
	})

	t.Run("message type template test send", func(t *testing.T) {
		beforeCalls := len(mockDiscord.ChannelCalls)

		createResp := postJSONWithCookie(t, router, "/api/notification-targets", adminCookie, map[string]any{
			"name":         "Template Test",
			"providerType": "discord",
			"config": map[string]string{
				"channelId": "template-channel",
			},
			"enabled": true,
		})
		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("create target status = %d body=%s", createResp.StatusCode, readBody(t, createResp))
		}
		var created struct {
			ID int64 `json:"id"`
		}
		decodeJSON(t, createResp, &created)

		targetsResp := putJSONWithCookie(t, router, "/api/message-types/player_joined/targets", adminCookie, map[string][]int64{
			"targetIds": {created.ID},
		})
		if targetsResp.StatusCode != http.StatusOK {
			t.Fatalf("put targets status = %d body=%s", targetsResp.StatusCode, readBody(t, targetsResp))
		}

		testTitle := "Test Send Title"
		testResp := postJSONWithCookie(t, router, "/api/message-types/player_joined/template/test", adminCookie, map[string]any{
			"variant": "embed",
			"template": map[string]any{
				"embed": map[string]any{
					"title": testTitle,
				},
			},
		})
		if testResp.StatusCode != http.StatusNoContent {
			t.Fatalf("template test status = %d body=%s", testResp.StatusCode, readBody(t, testResp))
		}
		if len(mockDiscord.ChannelCalls) <= beforeCalls {
			t.Fatal("expected discord send for template test")
		}
		gotTitle := mockDiscord.ChannelCalls[len(mockDiscord.ChannelCalls)-1].Message.Embeds[0].Title
		if gotTitle != testTitle {
			t.Fatalf("embed title = %q, want %q", gotTitle, testTitle)
		}

		var logCount int
		if err := database.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM notification_log
			WHERE message_type_key = 'player_joined' AND target_id = ? AND success = 1`,
			created.ID,
		).Scan(&logCount); err != nil {
			t.Fatalf("count notification_log: %v", err)
		}
		if logCount != 1 {
			t.Fatalf("notification_log rows = %d, want 1", logCount)
		}

		clearResp := putJSONWithCookie(t, router, "/api/message-types/player_joined/targets", adminCookie, map[string][]int64{
			"targetIds": {},
		})
		if clearResp.StatusCode != http.StatusOK {
			t.Fatalf("clear targets status = %d", clearResp.StatusCode)
		}

		noTargetsResp := postJSONWithCookie(t, router, "/api/message-types/player_joined/template/test", adminCookie, map[string]any{
			"variant": "embed",
			"template": map[string]any{
				"embed": map[string]any{
					"title": testTitle,
				},
			},
		})
		if noTargetsResp.StatusCode != http.StatusBadRequest {
			t.Fatalf("template test no targets status = %d, want 400 body=%s", noTargetsResp.StatusCode, readBody(t, noTargetsResp))
		}
		var errBody struct {
			Error string `json:"error"`
		}
		decodeJSON(t, noTargetsResp, &errBody)
		if errBody.Error != "no targets assigned" {
			t.Fatalf("error = %q, want no targets assigned", errBody.Error)
		}
	})
}

func setupAdmin(t *testing.T, router http.Handler) *http.Cookie {
	resp := postJSON(t, router, "/api/auth/setup", map[string]string{
		"username": "admin",
		"password": "secret123",
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
		INSERT INTO research_node_state (
			node_id, tree_name, name, category, state, tech_tier, cost_json,
			coord_x, coord_y, parents_json, updated_at
		)
		VALUES ('n1', 'MAM', 'Oil Processing', 'Oil', 'Purchased', 5,
			'[{"Name":"Iron Plate","Amount":100}]', 2, 3, '[{"x":1,"y":0}]', ?)`, now)
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

	_, err = database.ExecContext(ctx, `
		INSERT INTO circuit_snapshots (
			circuit_id, power_production, power_consumed, power_capacity, battery_percent, captured_at
		) VALUES (1, 500, 400, 1000, 50, ?)`, now)
	if err != nil {
		t.Fatalf("seed circuit snapshot: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO production_snapshots (
			item_class_name, item_display_name, produced_per_min, consumed_per_min, captured_at
		) VALUES ('Desc_Plastic_C', 'Plastic', 120, 80, ?)`, now)
	if err != nil {
		t.Fatalf("seed production snapshot: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO prod_stats_state (
			item_class_name, item_display_name, prod_per_min_label, prod_percent, cons_percent,
			current_prod, max_prod, current_consumed, max_consumed, transfer_type, updated_at
		) VALUES ('Desc_Plastic_C', 'Plastic', 'P: 120/min - C: 80/min', 75, 50, 120, 200, 80, 160, 'Belt', ?)`, now)
	if err != nil {
		t.Fatalf("seed prod stats: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO resource_sink_state (id, num_coupon, percent, points_to_coupon, total_points, updated_at)
		VALUES (1, 5, 42.5, 1000, 50000, ?)`, now)
	if err != nil {
		t.Fatalf("seed resource sink: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO resource_sink_snapshots (num_coupon, percent, total_points, captured_at)
		VALUES (5, 42.5, 50000, ?)`, now)
	if err != nil {
		t.Fatalf("seed resource sink snapshot: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO drone_state (
			drone_id, home_station, paired_station, has_paired_station, current_destination,
			flying_speed, max_speed, current_flying_mode, updated_at
		) VALUES ('d1', 'Home', 'Remote', 1, 'Remote', 25.5, 30, 'Flying', ?)`, now)
	if err != nil {
		t.Fatalf("seed drone: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO doggo_state (doggo_id, name, inventory_json, updated_at)
		VALUES ('dog1', 'Buddy', '[{"Name":"Iron Ore"}]', ?)`, now)
	if err != nil {
		t.Fatalf("seed doggo: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO train_state (
			train_id, name, derailed, pending_derail, status, self_driving_error,
			docking_status, path_status, station, updated_at
		) VALUES ('t1', 'Express', 0, 0, 'Self-Driving', 'SDLE_NoError', 'TDS_Docked', 'PDE_NoError', 'Station A', ?)`, now)
	if err != nil {
		t.Fatalf("seed train: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO vehicle_state (
			vehicle_id, vehicle_type, display_name, status, driver, autopilot, following_path,
			forward_speed, fuel_empty, low_speed_since, stuck, updated_at
		) VALUES ('v1', 'Explorer', 'Explorer', 'Active', 'Guggi', 1, 1, 45, 0, NULL, 0, ?)`, now)
	if err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}
}

func seedAdminAPIFixtures(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)

	_, err := database.ExecContext(ctx, `
		INSERT INTO notification_log (
			message_type_key, target_id, rendered_preview, success, error, sent_at,
			delivery_mode, recipient_external_user_id
		)
		VALUES ('player_joined', 1, 'Player Guggi joined', 1, NULL, ?, 'channel', NULL)`, now)
	if err != nil {
		t.Fatalf("seed notification log: %v", err)
	}

	unknownPhase := `[{"Name":"Unknown Item","ClassName":"Unknown_C"}]`
	_, err = database.ExecContext(ctx, `
		INSERT INTO elevator_phase_unknown_log (raw_current_phase_json, detected_at, resolved, resolved_at)
		VALUES (?, ?, 0, NULL)`, unknownPhase, now)
	if err != nil {
		t.Fatalf("seed elevator unknown log: %v", err)
	}
}

func newTestHandler(database *sql.DB, svc *auth.Service, regSvc *registration.Service, session notify.DiscordSession) *api.Handler {
	if session == nil {
		session = notify.NewMockDiscordSession()
	}
	provider := notify.NewDiscordProvider(session)
	connSvc := connection.NewService(database, provider)
	modsSvc := mods.NewService(database, func(ctx context.Context) (*frm.Client, error) {
		return frm.NewClient(frm.Config{Host: "127.0.0.1", Port: 1}), nil
	})
	return api.NewHandlerWithDiscordSession(database, svc, regSvc, connSvc, modsSvc, session)
}

func newMockFRMServer(t *testing.T, responses map[string][]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func frmServerHostPort(t *testing.T, srv *httptest.Server) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return host, port
}
