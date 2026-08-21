package planner

import (
	"math"
	"path/filepath"
	"testing"
)

func loadTestCatalog(t *testing.T) *Catalog {
	t.Helper()
	t.Chdir("../../..")
	path := filepath.Join("backend", "testdata", "planner", "factory_catalog.json")
	cat, err := loadCatalogFile(path)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	return cat
}

func TestSuggestIronPlate60(t *testing.T) {
	cat := loadTestCatalog(t)
	graph, err := Suggest(cat, SuggestRequest{
		ItemClass:           "Desc_IronPlate_C",
		RatePerMin:          60,
		DefaultClockPercent: 100,
	})
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	var constructors, smelters, oreSources float64
	var oreRate float64
	for _, node := range graph.Nodes {
		switch {
		case node.Role == "source" && node.ItemClass == "Desc_OreIron_C":
			oreSources++
			oreRate = node.RatePerMin
		case node.RecipeClass == "Recipe_IronPlate_C":
			constructors += node.Count
		case node.RecipeClass == "Recipe_IngotIron_C":
			smelters += node.Count
		}
	}

	if constructors < 2.9 || constructors > 3.1 {
		t.Fatalf("constructor count = %v, want ~3", constructors)
	}
	if smelters < 2.9 || smelters > 3.1 {
		t.Fatalf("smelter count = %v, want ~3", smelters)
	}
	if oreSources != 1 {
		t.Fatalf("ore sources = %v, want 1", oreSources)
	}
	if oreRate < 89 || oreRate > 91 {
		t.Fatalf("ore rate = %v, want ~90/min", oreRate)
	}

	var oreEdges int
	for _, edge := range graph.Edges {
		if edge.ItemClass == "Desc_OreIron_C" {
			oreEdges++
		}
	}
	if oreEdges != 1 {
		t.Fatalf("ore edges = %d, want 1", oreEdges)
	}
}

func TestSuggestPlastic60(t *testing.T) {
	cat := loadTestCatalog(t)
	graph, err := Suggest(cat, SuggestRequest{
		ItemClass:           "Desc_Plastic_C",
		RatePerMin:          60,
		DefaultClockPercent: 100,
	})
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	var refineries float64
	var oilRate float64
	var horEdges, plasticEdges int
	for _, node := range graph.Nodes {
		if node.RecipeClass == "Recipe_Plastic_C" {
			refineries += node.Count
		}
		if node.Role == "source" && node.ItemClass == "Desc_LiquidOil_C" {
			oilRate = node.RatePerMin
		}
	}
	for _, edge := range graph.Edges {
		switch edge.ItemClass {
		case "Desc_HeavyOilResidue_C":
			horEdges++
		case "Desc_Plastic_C":
			plasticEdges++
		}
	}

	if refineries < 2.9 || refineries > 3.1 {
		t.Fatalf("refinery count = %v, want 3", refineries)
	}
	if oilRate < 89 || oilRate > 91 {
		t.Fatalf("oil rate = %v, want 90 m³/min", oilRate)
	}
	if horEdges != 0 {
		t.Fatalf("HOR edges = %d, want 0", horEdges)
	}
	if plasticEdges != 0 {
		t.Fatalf("plastic edges = %d, want 0", plasticEdges)
	}
}

func TestSuggestCycleError(t *testing.T) {
	cat := &Catalog{
		Items: []Item{
			{ClassName: "Desc_A_C", Form: ItemFormSolid},
			{ClassName: "Desc_B_C", Form: ItemFormSolid},
		},
		Recipes: []Recipe{
			{
				ClassName:   "Recipe_A_C",
				DurationSec: 1,
				Ingredients: []ItemAmount{{ClassName: "Desc_B_C", Amount: 1}},
				Products:    []ItemAmount{{ClassName: "Desc_A_C", Amount: 1}},
				ProducedIn:  []string{"Build_ConstructorMk1_C"},
			},
			{
				ClassName:   "Recipe_B_C",
				DurationSec: 1,
				Ingredients: []ItemAmount{{ClassName: "Desc_A_C", Amount: 1}},
				Products:    []ItemAmount{{ClassName: "Desc_B_C", Amount: 1}},
				ProducedIn:  []string{"Build_ConstructorMk1_C"},
			},
		},
		Buildings: []Building{{
			ClassName:          "Build_ConstructorMk1_C",
			ManufacturingSpeed: 1,
		}},
	}
	cat.buildIndexes(false)

	_, err := Suggest(cat, SuggestRequest{ItemClass: "Desc_A_C", RatePerMin: 10})
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var cycle *CycleError
	if c, ok := err.(*CycleError); !ok {
		t.Fatalf("error = %v, want CycleError", err)
	} else {
		cycle = c
	}
	if len(cycle.Path) == 0 {
		t.Fatalf("cycle path = %v", cycle.Path)
	}
}

func TestRoundCount(t *testing.T) {
	if got := RoundCount(3.333333); math.Abs(got-3.33) > 0.001 {
		t.Fatalf("RoundCount = %v", got)
	}
}
