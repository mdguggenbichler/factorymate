package planner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type powerExample struct {
	ID               string  `json:"id"`
	BuildingClass    string  `json:"buildingClass"`
	PowerBaseMW      float64 `json:"powerBaseMW"`
	ClockPercent     float64 `json:"clockPercent"`
	SomersloopCount  int     `json:"somersloopCount"`
	SomersloopSlots  int     `json:"somersloopSlots"`
	PowerExponent    float64 `json:"powerExponent"`
	ExpectedMW       float64 `json:"expectedMW"`
}

func TestPowerExamplesGolden(t *testing.T) {
	t.Chdir("../../..")
	path := filepath.Join("backend", "testdata", "planner", "power_examples.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	var examples []powerExample
	if err := json.Unmarshal(raw, &examples); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, ex := range examples {
		got := ComputePowerMW(ex.PowerBaseMW, ex.ClockPercent, ex.PowerExponent, ex.SomersloopCount, ex.SomersloopSlots)
		if !WithinPowerTolerance(got, ex.ExpectedMW) {
			t.Fatalf("%s: got %.4f MW, want %.2f MW", ex.ID, got, ex.ExpectedMW)
		}
	}
}

func TestBuildingPowerFromCatalog(t *testing.T) {
	t.Chdir("../../..")
	cat, err := LoadCatalogFromDocs(
		filepath.Join("docs", "FactoryGame-Docs.json"),
		filepath.Join("assets", "icons.json"),
	)
	if err != nil {
		t.Fatalf("LoadCatalogFromDocs: %v", err)
	}

	cases := []struct {
		id      string
		build   string
		clock   float64
		sloops  int
		want    float64
	}{
		{"constructor_150", "Build_ConstructorMk1_C", 150, 0, 6.84},
		{"assembler_1_sloop", "Build_AssemblerMk1_C", 100, 1, 33.75},
		{"constructor_1_sloop", "Build_ConstructorMk1_C", 100, 1, 16},
		{"constructor_250_sloop", "Build_ConstructorMk1_C", 250, 1, 53.7},
	}

	for _, tc := range cases {
		got, ok := BuildingPowerMW(cat, tc.build, tc.clock, tc.sloops)
		if !ok {
			t.Fatalf("%s: building not found", tc.id)
		}
		if !WithinPowerTolerance(got, tc.want) {
			t.Fatalf("%s: got %.4f MW, want %.2f MW", tc.id, got, tc.want)
		}
	}
}

func TestAnalyzeGraphBalanceFixture(t *testing.T) {
	t.Chdir("../../..")
	cat, err := LoadCatalogFromDocs(
		filepath.Join("docs", "FactoryGame-Docs.json"),
		filepath.Join("assets", "icons.json"),
	)
	if err != nil {
		t.Fatalf("LoadCatalogFromDocs: %v", err)
	}

	path := filepath.Join("backend", "testdata", "planner", "balance_iron_plate.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture struct {
		Graph    Graph                  `json:"graph"`
		Expected map[string]EdgeBalance `json:"expectedEdges"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	result := cat.AnalyzeGraph(fixture.Graph)
	for edgeID, want := range fixture.Expected {
		got, ok := result.Edges[edgeID]
		if !ok {
			t.Fatalf("missing edge %s", edgeID)
		}
		if !WithinPowerTolerance(got.FlowPerMin, want.FlowPerMin) {
			t.Fatalf("edge %s flow = %.4f, want %.4f", edgeID, got.FlowPerMin, want.FlowPerMin)
		}
		if got.RecommendedMk != want.RecommendedMk {
			t.Fatalf("edge %s mk = %d, want %d", edgeID, got.RecommendedMk, want.RecommendedMk)
		}
		if got.ExceedsMax != want.ExceedsMax {
			t.Fatalf("edge %s exceedsMax = %v, want %v", edgeID, got.ExceedsMax, want.ExceedsMax)
		}
	}
}

func TestRecommendTransportMk(t *testing.T) {
	t.Chdir("../../..")
	cat, err := LoadCatalogFromDocs(
		filepath.Join("docs", "FactoryGame-Docs.json"),
		filepath.Join("assets", "icons.json"),
	)
	if err != nil {
		t.Fatalf("LoadCatalogFromDocs: %v", err)
	}

	solid := cat.RecommendBeltMk(60)
	if solid.Mk != 1 || solid.ExceedsMax {
		t.Fatalf("solid mk = %+v", solid)
	}

	fluid := cat.RecommendPipeMk(300)
	if fluid.Mk != 1 || fluid.ExceedsMax {
		t.Fatalf("fluid mk = %+v", fluid)
	}
}
