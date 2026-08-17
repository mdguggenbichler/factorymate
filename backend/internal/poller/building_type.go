package poller

import "factorymate/internal/frm"

// Verified ClassName → building_type mappings (spec §4.1).
// Doc-sourced: Constructor, Refinery. Live FRM (192.168.178.42:8889): Assembler, Foundry, Smelter.
var classNameToBuildingType = map[string]string{
	"Build_ConstructorMk1_C": "Constructor",
	"Build_OilRefinery_C":    "Refinery",
	"Build_AssemblerMk1_C":   "Assembler",
	"Build_FoundryMk1_C":     "Foundry",
	"Build_SmelterMk1_C":     "Smelter",
}

// buildingTypeFromMachine derives factory_machine_state.building_type per spec §4.1.
// Unmapped ClassNames fall back to FRM features.properties.type when present.
func buildingTypeFromMachine(m frm.FactoryMachine) string {
	if t, ok := classNameToBuildingType[m.ClassName]; ok {
		return t
	}
	if t := m.BuildingType(); t != "" {
		return t
	}
	return "Unknown"
}
