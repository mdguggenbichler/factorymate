package frm

import "encoding/json"

// --- Fast poll types ---

// Player is a FRM getPlayer entry.
type Player struct {
	ID     string `json:"ID"`
	Name   string `json:"Name"`
	Online bool   `json:"Online"`
}

// Circuit is a FRM getPower entry.
type Circuit struct {
	CircuitGroupID      int     `json:"CircuitGroupID"`
	FuseTriggered       bool    `json:"FuseTriggered"`
	PowerProduction     float64 `json:"PowerProduction"`
	PowerConsumed       float64 `json:"PowerConsumed"`
	PowerCapacity       float64 `json:"PowerCapacity"`
	PowerMaxConsumed    float64 `json:"PowerMaxConsumed"`
	BatteryDifferential float64 `json:"BatteryDifferential"`
	BatteryPercent      float64 `json:"BatteryPercent"`
	BatteryCapacity     float64 `json:"BatteryCapacity"`
	BatteryTimeEmpty    string  `json:"BatteryTimeEmpty"`
	BatteryTimeFull     string  `json:"BatteryTimeFull"`
}

// Recipe is a schematic recipe summary.
type Recipe struct {
	Name      string `json:"Name"`
	ClassName string `json:"ClassName"`
}

// Schematic is a FRM getSchematics entry.
type Schematic struct {
	ID        string   `json:"ID"`
	Name      string   `json:"Name"`
	Type      string   `json:"Type"`
	Purchased bool     `json:"Purchased"`
	Locked    bool     `json:"Locked"`
	TechTier  int      `json:"TechTier"`
	Recipes   []Recipe `json:"Recipes"`
}

// PhaseItem is a space elevator current-phase requirement.
type PhaseItem struct {
	Name          string `json:"Name"`
	ClassName     string `json:"ClassName"`
	Amount        int    `json:"Amount"`
	MaxAmount     int    `json:"MaxAmount"`
	RemainingCost int    `json:"RemainingCost"`
	TotalCost     int    `json:"TotalCost"`
}

// Elevator is a FRM getSpaceElevator entry.
type Elevator struct {
	ID           string      `json:"ID"`
	Name         string      `json:"Name"`
	CurrentPhase []PhaseItem `json:"CurrentPhase"`
	UpgradeReady bool        `json:"UpgradeReady"`
}

// Item is a generic FRM inventory/cost item.
type Item struct {
	Name      string `json:"Name"`
	ClassName string `json:"ClassName"`
	Amount    int    `json:"Amount"`
	MaxAmount int    `json:"MaxAmount"`
}

// ResearchNode is a M.A.M. research node.
type ResearchNode struct {
	ID        string `json:"ID"`
	Name      string `json:"Name"`
	ClassName string `json:"ClassName"`
	Category  string `json:"Category"`
	State     string `json:"State"`
	TechTier  int    `json:"TechTier"`
	Cost      []Item `json:"Cost"`
}

// ResearchTree is a FRM getResearchTrees entry.
type ResearchTree struct {
	Name  string         `json:"Name"`
	Nodes []ResearchNode `json:"Nodes"`
}

// Train is a FRM getTrains entry (field names match FRM exactly per §4.1.1).
type Train struct {
	ID            string `json:"ID"`
	Name          string `json:"Name"`
	Derailed      bool   `json:"Derailed"`
	PendingDerail bool   `json:"PendingDerail"`
	Status        string `json:"Status"`
	TrainStation  string `json:"TrainStation"`
	SelfDriving   string `json:"SelfDriving"`
	Docking       string `json:"Docking"`
	Path          string `json:"Path"`
}

// Vehicle is a FRM getVehicles entry.
type Vehicle struct {
	ID            FlexibleID `json:"ID"`
	VehicleType   string     `json:"VehicleType"`
	Status        string     `json:"Status"`
	Driver        string     `json:"Driver"`
	AutoPilot     bool       `json:"AutoPilot"`
	Autopilot     bool       `json:"Autopilot"` // live FRM spelling variant
	FollowingPath bool       `json:"FollowingPath"`
	ForwardSpeed  float64    `json:"ForwardSpeed"`
	Fuel          []Item     `json:"Fuel"`
	FuelInventory []Item     `json:"FuelInventory"`
	Features      struct {
		Properties struct {
			Type string `json:"type"`
		} `json:"properties"`
	} `json:"features"`
}

// Type returns the vehicle type from VehicleType or features.properties.type.
func (v Vehicle) Type() string {
	if v.VehicleType != "" {
		return v.VehicleType
	}
	return v.Features.Properties.Type
}

// IsAutoPilot reports whether autopilot is engaged (handles both JSON spellings).
func (v Vehicle) IsAutoPilot() bool {
	return v.AutoPilot || v.Autopilot
}

// Fuels returns fuel items from Fuel or FuelInventory (live FRM uses FuelInventory).
func (v Vehicle) Fuels() []Item {
	if len(v.Fuel) > 0 {
		return v.Fuel
	}
	return v.FuelInventory
}

// --- Slow poll types ---

// ProdStat is a FRM getProdStats entry (maps to prod_stats_state per §4.1.1).
type ProdStat struct {
	Name            string  `json:"Name"`
	ClassName       string  `json:"ClassName"`
	ProdPerMin      string  `json:"ProdPerMin"`
	ProdPercent     float64 `json:"ProdPercent"`
	ConsPercent     float64 `json:"ConsPercent"`
	CurrentProd     float64 `json:"CurrentProd"`
	MaxProd         float64 `json:"MaxProd"`
	CurrentConsumed float64 `json:"CurrentConsumed"`
	MaxConsumed     float64 `json:"MaxConsumed"`
	Type            string  `json:"Type"`
}

// ResourceSink is the first element of a getResourceSink array response.
type ResourceSink struct {
	NumCoupon      int               `json:"NumCoupon"`
	Percent        float64           `json:"Percent"`
	PointsToCoupon int               `json:"PointsToCoupon"`
	TotalPoints    int               `json:"TotalPoints"`
	GraphPoints    []json.RawMessage `json:"GraphPoints"` // Value/value/number — ignored for history
}

// FactoryFlowItem is an ingredient or production line on a factory machine.
type FactoryFlowItem struct {
	Name            string         `json:"Name"`
	ClassName       string         `json:"ClassName"`
	Amount          FlexibleAmount `json:"Amount"`
	CurrentProd     float64        `json:"CurrentProd"`
	MaxProd         float64        `json:"MaxProd"`
	ProdPercent     float64        `json:"ProdPercent"`
	CurrentConsumed float64        `json:"CurrentConsumed"`
	MaxConsumed     float64        `json:"MaxConsumed"`
	ConsPercent     float64        `json:"ConsPercent"`
}

// PowerInfo is nested power data on factory buildings and trains.
type PowerInfo struct {
	CircuitGroupID   int     `json:"CircuitGroupID"`
	CircuitID        int     `json:"CircuitID"`
	FuseTriggered    bool    `json:"FuseTriggered"`
	PowerConsumed    float64 `json:"PowerConsumed"`
	MaxPowerConsumed float64 `json:"MaxPowerConsumed"`
}

// FactoryMachine is a FRM getFactory entry.
type FactoryMachine struct {
	ID           string            `json:"ID"`
	Name         string            `json:"Name"`
	ClassName    string            `json:"ClassName"`
	Recipe       string            `json:"Recipe"`
	ManuSpeed    float64           `json:"ManuSpeed"`
	IsConfigured bool              `json:"IsConfigured"`
	IsProducing  bool              `json:"IsProducing"`
	IsPaused     bool              `json:"IsPaused"`
	Ingredients  []FactoryFlowItem `json:"ingredients"`
	Production   []FactoryFlowItem `json:"production"`
	PowerInfo    PowerInfo         `json:"PowerInfo"`
	Features     struct {
		Properties struct {
			Type string `json:"type"`
		} `json:"properties"`
	} `json:"features"`
}

// BuildingType returns FRM's features.properties.type when set.
func (m FactoryMachine) BuildingType() string {
	return m.Features.Properties.Type
}

// Drone is a FRM getDrone entry.
type Drone struct {
	ID                  string        `json:"ID"`
	Name                string        `json:"Name"`
	HomeStation         string        `json:"HomeStation"`
	PairedStation       string        `json:"PairedStation"`
	HasPairedStation    bool          `json:"HasPairedStation"`
	CurrentDestination  string        `json:"CurrentDestination"`
	FlyingSpeed         FlexibleFloat `json:"FlyingSpeed"`
	MaxSpeed            FlexibleFloat `json:"MaxSpeed"`
	CurrentFlyingMode   string        `json:"CurrentFlyingMode"`
}

// Doggo is a FRM getDoggo entry.
type Doggo struct {
	ID        string `json:"ID"`
	Name      string `json:"Name"`
	Inventory []Item `json:"Inventory"`
}
