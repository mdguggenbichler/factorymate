package planner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseUEItemList(t *testing.T) {
	raw := `((ItemClass="/Script/Engine.BlueprintGeneratedClass'/Game/FactoryGame/Resource/Parts/Plastic/Desc_Plastic.Desc_Plastic_C'",Amount=2),(ItemClass="/Script/Engine.BlueprintGeneratedClass'/Game/FactoryGame/Resource/Parts/HeavyOilResidue/Desc_HeavyOilResidue.Desc_HeavyOilResidue_C'",Amount=1000))`
	items, err := ParseUEItemList(raw)
	if err != nil {
		t.Fatalf("ParseUEItemList: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].ClassName != "Desc_Plastic_C" || items[0].Amount != 2 {
		t.Fatalf("first item = %+v", items[0])
	}
	if items[1].ClassName != "Desc_HeavyOilResidue_C" || items[1].Amount != 1000 {
		t.Fatalf("second item = %+v", items[1])
	}
}

func TestParseUEClassListFiltersWorkbench(t *testing.T) {
	raw := `("/Game/FactoryGame/Buildable/Factory/ConstructorMk1/Build_ConstructorMk1.Build_ConstructorMk1_C","/Game/FactoryGame/Buildable/-Shared/WorkBench/BP_WorkBenchComponent.BP_WorkBenchComponent_C")`
	all := ParseUEClassList(raw)
	filtered := filterProducedIn(all)
	if len(filtered) != 1 || filtered[0] != "Build_ConstructorMk1_C" {
		t.Fatalf("filtered = %v", filtered)
	}
}

func TestDecodeUTF16Fixture(t *testing.T) {
	t.Chdir("../../..")
	path := filepath.Join("backend", "testdata", "planner", "utf16_sample.json")
	groups, err := decodeDocsJSON(path)
	if err != nil {
		t.Fatalf("decodeDocsJSON: %v", err)
	}
	if len(groups) != 1 || len(groups[0].Classes) != 1 {
		t.Fatalf("unexpected groups: %+v", groups)
	}
	if classString(groups[0].Classes[0], "ClassName") != "Recipe_Test_C" {
		t.Fatalf("class = %q", classString(groups[0].Classes[0], "ClassName"))
	}
}

func TestFactoryDumpHasUTF16BOM(t *testing.T) {
	t.Chdir("../../..")
	path := FactoryGameDocsRelPath
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("dump not available: %v", err)
	}
	if len(raw) < 2 || raw[0] != 0xFF || raw[1] != 0xFE {
		t.Fatalf("first bytes = % x, want ff fe", raw[:2])
	}
}

func TestLoadCatalogFromDocs(t *testing.T) {
	t.Chdir("../../..")
	cat, err := LoadCatalogFromDocs(
		FactoryGameDocsRelPath,
		filepath.Join("assets", "icons.json"),
	)
	if err != nil {
		t.Fatalf("LoadCatalogFromDocs: %v", err)
	}
	if len(cat.Items) == 0 || len(cat.Recipes) == 0 || len(cat.Buildings) == 0 {
		t.Fatalf("empty catalog: items=%d recipes=%d buildings=%d", len(cat.Items), len(cat.Recipes), len(cat.Buildings))
	}

	plastic, ok := cat.recipesByClass["Recipe_Plastic_C"]
	if !ok {
		t.Fatal("missing Recipe_Plastic_C")
	}
	var oilAmount float64
	for _, ing := range plastic.Ingredients {
		if ing.ClassName == "Desc_LiquidOil_C" {
			oilAmount = ing.Amount
		}
	}
	if oilAmount != 3 {
		t.Fatalf("plastic oil amount = %v, want 3", oilAmount)
	}

	alt, ok := cat.recipesByClass["Recipe_Alternate_Turbofuel_C"]
	if !ok || !alt.IsAlternate {
		t.Fatalf("alternate recipe detection failed: ok=%v alt=%+v", ok, alt)
	}

	smelter, ok := cat.BuildingByClass("Build_SmelterMk1_C")
	if !ok || smelter.SomersloopSlots != 1 {
		t.Fatalf("smelter slots = %d, want override 1", smelter.SomersloopSlots)
	}
	packager, ok := cat.BuildingByClass("Build_Packager_C")
	if !ok || packager.SomersloopSlots != 0 {
		t.Fatalf("packager slots = %d, want 0", packager.SomersloopSlots)
	}

	var mk1Rate float64
	for _, belt := range cat.Belts {
		if belt.ClassName == "Build_ConveyorBeltMk1_C" {
			mk1Rate = belt.ItemsPerMin
		}
	}
	if mk1Rate != 60 {
		t.Fatalf("mk1 belt rate = %v, want 60", mk1Rate)
	}

	var pipeMk1 float64
	for _, pipe := range cat.Pipes {
		if pipe.ClassName == "Build_Pipeline_C" {
			pipeMk1 = pipe.CubicMetersPerMin
		}
	}
	if pipeMk1 != 300 {
		t.Fatalf("mk1 pipe rate = %v, want 300", pipeMk1)
	}

	iconClass := cat.ResolveIconClassName("Build_ConstructorMk1_C")
	if iconClass != "Desc_ConstructorMk1_C" {
		t.Fatalf("icon class = %q", iconClass)
	}
}

func TestLoadCatalogFromSlimFile(t *testing.T) {
	t.Chdir("../../..")
	path := filepath.Join("backend", "testdata", "planner", "factory_catalog.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("slim catalog not generated yet")
	}
	cat, err := loadCatalogFile(path)
	if err != nil {
		t.Fatalf("loadCatalogFile: %v", err)
	}
	if len(cat.Recipes) == 0 {
		t.Fatal("expected recipes in slim catalog")
	}
}

func TestSlimCatalogJSONRoundTrip(t *testing.T) {
	t.Chdir("../../..")
	cat, err := LoadCatalogFromDocs(
		FactoryGameDocsRelPath,
		filepath.Join("assets", "icons.json"),
	)
	if err != nil {
		t.Fatalf("LoadCatalogFromDocs: %v", err)
	}
	raw, err := json.Marshal(cat)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Catalog
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	decoded.buildIndexes(false)
	if len(decoded.Recipes) != len(cat.Recipes) {
		t.Fatalf("recipe count mismatch")
	}
}
