package poller

import (
	"testing"

	"factorymate/internal/frm"
)

func TestBuildingTypeFromMachine(t *testing.T) {
	tests := []struct {
		className string
		features  string
		want      string
	}{
		{"Build_ConstructorMk1_C", "", "Constructor"},
		{"Build_OilRefinery_C", "", "Refinery"},
		{"Build_AssemblerMk1_C", "", "Assembler"},
		{"Build_FoundryMk1_C", "", "Foundry"},
		{"Build_SmelterMk1_C", "", "Smelter"},
		{"Build_ManufacturerMk1_C", "Manufacturer", "Manufacturer"},
		{"Build_Unknown_C", "", "Unknown"},
	}

	for _, tc := range tests {
		m := frm.FactoryMachine{ClassName: tc.className}
		m.Features.Properties.Type = tc.features
		got := buildingTypeFromMachine(m)
		if got != tc.want {
			t.Errorf("buildingTypeFromMachine(%q, feature=%q) = %q, want %q",
				tc.className, tc.features, got, tc.want)
		}
	}
}
