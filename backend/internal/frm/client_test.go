package frm

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "frm", name)
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestParseFixture_getPlayer(t *testing.T) {
	var players []Player
	if err := json.Unmarshal(readFixture(t, "getPlayer.json"), &players); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(players) == 0 {
		t.Fatal("expected at least one player")
	}
	if players[0].ID == "" || players[0].Name == "" {
		t.Fatalf("unexpected player: %+v", players[0])
	}
}

func TestParseFixture_getPower(t *testing.T) {
	var circuits []Circuit
	if err := json.Unmarshal(readFixture(t, "getPower.json"), &circuits); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(circuits) == 0 {
		t.Fatal("expected at least one circuit")
	}
}

func TestParseFixture_getSchematics(t *testing.T) {
	var schematics []Schematic
	if err := json.Unmarshal(readFixture(t, "getSchematics.json"), &schematics); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(schematics) == 0 {
		t.Fatal("expected at least one schematic")
	}
	if len(schematics[0].Recipes) == 0 {
		t.Fatal("expected schematic with recipes")
	}
}

func TestParseFixture_getSpaceElevator(t *testing.T) {
	var elevators []Elevator
	if err := json.Unmarshal(readFixture(t, "getSpaceElevator.json"), &elevators); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(elevators) == 0 || len(elevators[0].CurrentPhase) == 0 {
		t.Fatalf("unexpected elevator: %+v", elevators)
	}
}

func TestParseFixture_getResearchTrees(t *testing.T) {
	var trees []ResearchTree
	if err := json.Unmarshal(readFixture(t, "getResearchTrees.json"), &trees); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(trees) == 0 || len(trees[0].Nodes) == 0 {
		t.Fatal("expected research tree with nodes")
	}
	node := trees[0].Nodes[0]
	if len(node.Cost) == 0 {
		t.Fatal("expected node with cost items")
	}
	if node.Coordinates == nil || node.Coordinates.X != 4 || node.Coordinates.Y != 4 {
		t.Fatalf("coordinates = %+v, want (4,4)", node.Coordinates)
	}
	if len(node.Parents) != 1 || node.Parents[0].X != 3 || node.Parents[0].Y != 3 {
		t.Fatalf("parents = %+v, want [{3,3}]", node.Parents)
	}
}

func TestParseFixture_getTrains(t *testing.T) {
	var trains []Train
	if err := json.Unmarshal(readFixture(t, "getTrains.json"), &trains); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestParseFixture_getVehicles(t *testing.T) {
	var vehicles []Vehicle
	if err := json.Unmarshal(readFixture(t, "getVehicles.json"), &vehicles); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(vehicles) < 2 {
		t.Fatalf("expected 2 vehicles, got %d", len(vehicles))
	}
	if vehicles[0].Type() != "Tractor" {
		t.Fatalf("vehicle type = %q, want Tractor", vehicles[0].Type())
	}
	if len(vehicles[0].Fuels()) == 0 {
		t.Fatal("expected fuel from FuelInventory")
	}
	if !vehicles[1].IsAutoPilot() {
		t.Fatal("second vehicle should have autopilot engaged")
	}
}

func TestParseFixture_getProdStats(t *testing.T) {
	var stats []ProdStat
	if err := json.Unmarshal(readFixture(t, "getProdStats.json"), &stats); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(stats) == 0 {
		t.Fatal("expected prod stats")
	}
}

func TestParseFixture_getResourceSink(t *testing.T) {
	var sinks []ResourceSink
	if err := json.Unmarshal(readFixture(t, "getResourceSink.json"), &sinks); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sinks) == 0 {
		t.Fatal("expected sink array")
	}
	if sinks[0].NumCoupon == 0 && sinks[0].TotalPoints == 0 {
		t.Fatalf("unexpected sink: %+v", sinks[0])
	}
}

func TestParseFixture_getFactory(t *testing.T) {
	var machines []FactoryMachine
	if err := json.Unmarshal(readFixture(t, "getFactory.json"), &machines); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(machines) == 0 {
		t.Fatal("expected factory machines")
	}
	if len(machines[0].Ingredients) == 0 || len(machines[0].Production) == 0 {
		t.Fatal("expected ingredients and production arrays")
	}
	if machines[0].Ingredients[0].Amount.Value == "" {
		t.Fatal("expected parsed ingredient amount")
	}
}

func TestParseFixture_getDrone(t *testing.T) {
	var drones []Drone
	if err := json.Unmarshal(readFixture(t, "getDrone.json"), &drones); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestParseFixture_getDoggo(t *testing.T) {
	var doggos []Doggo
	if err := json.Unmarshal(readFixture(t, "getDoggo.json"), &doggos); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doggos) == 0 || len(doggos[0].Inventory) == 0 {
		t.Fatalf("unexpected doggo: %+v", doggos)
	}
}

func TestResourceSink_graphPointVariants(t *testing.T) {
	cases := []string{
		`[{"NumCoupon":1,"Percent":0.5,"PointsToCoupon":10,"TotalPoints":100,"GraphPoints":[1,2,3]}]`,
		`[{"NumCoupon":1,"Percent":0.5,"PointsToCoupon":10,"TotalPoints":100,"GraphPoints":[{"Value":1},{"value":2}]}]`,
	}
	for i, raw := range cases {
		var sinks []ResourceSink
		if err := json.Unmarshal([]byte(raw), &sinks); err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
	}
}

func TestGetSessionInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getSessionInfo" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"SessionName":"GuggiRaid Factory","IsPaused":false}`))
	}))
	t.Cleanup(srv.Close)

	host, port, err := splitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(Config{Host: host, Port: port})
	info, err := client.GetSessionInfo(context.Background())
	if err != nil {
		t.Fatalf("GetSessionInfo: %v", err)
	}
	if info.SessionName != "GuggiRaid Factory" {
		t.Fatalf("SessionName = %q", info.SessionName)
	}
}

func TestClient_authHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("X-FRM-Authorization")
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)

	host, port, err := splitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(Config{Host: host, Port: port, Token: "secret-token"})
	var players []Player
	if err := client.get(context.Background(), "getPlayer", &players); err != nil {
		t.Fatalf("get: %v", err)
	}
	if gotAuth != "secret-token" {
		t.Fatalf("auth header = %q, want secret-token", gotAuth)
	}
}

func TestFlexibleTypes(t *testing.T) {
	var id FlexibleID
	if err := json.Unmarshal([]byte(`42`), &id); err != nil || id.String() != "42" {
		t.Fatalf("flexible id int: %v %q", err, id)
	}
	if err := json.Unmarshal([]byte(`"abc"`), &id); err != nil || id.String() != "abc" {
		t.Fatalf("flexible id string: %v %q", err, id)
	}

	var speed FlexibleFloat
	if err := json.Unmarshal([]byte(`12.5`), &speed); err != nil || speed.Float64() != 12.5 {
		t.Fatalf("flexible float number: %v", err)
	}
	if err := json.Unmarshal([]byte(`"3.14"`), &speed); err != nil || speed.Float64() != 3.14 {
		t.Fatalf("flexible float string: %v", err)
	}

	var amt FlexibleAmount
	if err := json.Unmarshal([]byte(`6`), &amt); err != nil || amt.Value != "6" {
		t.Fatalf("flexible amount int: %v %+v", err, amt)
	}
	if err := json.Unmarshal([]byte(`"12"`), &amt); err != nil || amt.Value != "12" {
		t.Fatalf("flexible amount string: %v %+v", err, amt)
	}
}

func TestConfig_baseURL(t *testing.T) {
	cfg := Config{Host: "192.168.1.1", Port: 8889}
	if got := cfg.BaseURL(); got != "http://192.168.1.1:8889" {
		t.Fatalf("BaseURL = %q", got)
	}
}

func TestFastSlowEndpointLists(t *testing.T) {
	if len(FastEndpoints()) != 7 {
		t.Fatalf("fast endpoints = %d, want 7", len(FastEndpoints()))
	}
	if len(SlowEndpoints()) != 5 {
		t.Fatalf("slow endpoints = %d, want 5", len(SlowEndpoints()))
	}
}

func TestGetFast_partialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getPlayer":
			_, _ = w.Write([]byte(`[{"ID":"p1","Name":"Alice","Online":true}]`))
		case "/getPower":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte("[]"))
		}
	}))
	t.Cleanup(srv.Close)

	host, port, err := splitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(Config{Host: host, Port: port})
	result := client.GetFast(context.Background())

	if result.Reachable() {
		t.Fatal("expected unreachable when getPower fails")
	}
	if _, ok := result.Errors["getPower"]; !ok {
		t.Fatalf("expected getPower error, got %v", result.Errors)
	}
	if len(result.Players) != 1 {
		t.Fatalf("players = %d, want 1", len(result.Players))
	}
}

func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}
