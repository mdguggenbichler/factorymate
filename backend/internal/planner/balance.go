package planner

import (
	"math"
	"sort"
	"strings"
)

const powerToleranceMW = 0.01

// NodeRates holds per-item production/consumption rates for one graph node.
type NodeRates struct {
	Outputs map[string]float64 `json:"outputs"`
	Inputs  map[string]float64 `json:"inputs"`
	PowerMW float64            `json:"powerMW"`
}

// EdgeBalance is derived flow data for one connection.
type EdgeBalance struct {
	ItemClass       string  `json:"itemClass"`
	FlowPerMin      float64 `json:"flowPerMin"`
	DemandPerMin    float64 `json:"demandPerMin"`
	SupplyPerMin    float64 `json:"supplyPerMin"`
	OverPerMin      float64 `json:"overPerMin"`
	UnderPerMin     float64 `json:"underPerMin"`
	RecommendedMk   int     `json:"recommendedMk"`
	CapacityPerMin  float64 `json:"capacityPerMin"`
	ExceedsMax      bool    `json:"exceedsMax"`
	Unit            string  `json:"unit"`
}

// MkRecommendation is belt/pipe Mk guidance for a numeric rate.
type MkRecommendation struct {
	Mk             int     `json:"mk"`
	RatePerMin     float64 `json:"ratePerMin"`
	CapacityPerMin float64 `json:"capacityPerMin"`
	ExceedsMax     bool    `json:"exceedsMax"`
	Unit           string  `json:"unit"`
}

// GraphNode is the persisted process/source node shape used by balance.
type GraphNode struct {
	ID              string  `json:"id"`
	Role            string  `json:"role"`
	RecipeClass     string  `json:"recipeClass"`
	BuildingClass   string  `json:"buildingClass"`
	ItemClass       string  `json:"itemClass"`
	Count           float64 `json:"count"`
	ClockPercent    float64 `json:"clockPercent"`
	SomersloopCount int     `json:"somersloopCount"`
	RatePerMin      float64 `json:"ratePerMin"`
}

// GraphEdge connects two node ports.
type GraphEdge struct {
	ID            string `json:"id"`
	SourceNodeID  string `json:"sourceNodeId"`
	SourcePort    string `json:"sourcePort"`
	TargetNodeID  string `json:"targetNodeId"`
	TargetPort    string `json:"targetPort"`
	ItemClass     string `json:"itemClass"`
}

// Graph is the minimal graph_json shape for balance analysis.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// BalanceResult is the derived balance output for a graph.
type BalanceResult struct {
	Nodes map[string]NodeRates          `json:"nodes"`
	Edges map[string]EdgeBalance        `json:"edges"`
	TotalPowerMW float64                `json:"totalPowerMW"`
}

// ComputePowerMW returns building power draw using wiki formula from proposal §4.1.
func ComputePowerMW(powerBaseMW, clockPercent float64, powerExponent float64, somersloopCount, slotCount int) float64 {
	if powerBaseMW <= 0 {
		return 0
	}
	clockFactor := math.Pow(clockPercent/100, powerExponent)
	power := powerBaseMW * clockFactor
	if slotCount > 0 && somersloopCount > 0 {
		sloopFactor := math.Pow(1+float64(somersloopCount)/float64(slotCount), 2)
		power *= sloopFactor
	}
	return power
}

// BuildingPowerMW computes power for a catalog building with overrides.
func BuildingPowerMW(cat *Catalog, buildingClass string, clockPercent float64, somersloopCount int) (float64, bool) {
	building, ok := cat.BuildingByClass(buildingClass)
	if !ok {
		return 0, false
	}
	slots := building.SomersloopSlots
	if !building.CanChangeProductionBoost {
		somersloopCount = 0
	}
	return ComputePowerMW(building.PowerBaseMW, clockPercent, building.PowerExponent, somersloopCount, slots), true
}

// SomersloopOutputMultiplier returns the production boost from filled sloops.
func SomersloopOutputMultiplier(building *Building, somersloopCount int) float64 {
	if building == nil || !building.CanChangeProductionBoost || building.SomersloopSlots <= 0 {
		return 1
	}
	if somersloopCount <= 0 {
		return 1
	}
	return 1 + float64(somersloopCount)*building.SomersloopBoostMultiplier
}

// RecipeOutputPerMinute returns the per-building output rate for one product.
func RecipeOutputPerMinute(cat *Catalog, recipeClass, productClass, buildingClass string, clockPercent float64, somersloopCount int) (float64, bool) {
	recipe, ok := cat.recipesByClass[recipeClass]
	if !ok {
		return 0, false
	}
	building, ok := cat.BuildingByClass(buildingClass)
	if !ok {
		return 0, false
	}

	var amount float64
	for _, product := range recipe.Products {
		if product.ClassName == productClass {
			amount = product.Amount
			break
		}
	}
	if amount <= 0 {
		return 0, false
	}

	cyclesPerMin := (60 / recipe.DurationSec) * building.ManufacturingSpeed * (clockPercent / 100)
	rate := cyclesPerMin * amount * SomersloopOutputMultiplier(building, somersloopCount)
	return rate, true
}

// RecommendBeltMk returns the smallest belt Mk whose capacity covers rate items/min.
func (c *Catalog) RecommendBeltMk(ratePerMin float64) MkRecommendation {
	unit := "items/min"
	if len(c.Belts) == 0 {
		return MkRecommendation{RatePerMin: ratePerMin, Unit: unit, ExceedsMax: ratePerMin > 0}
	}

	sorted := append([]Belt(nil), c.Belts...)
	sort.Slice(sorted, func(i, j int) bool {
		return beltMkNumber(sorted[i].ClassName) < beltMkNumber(sorted[j].ClassName)
	})

	maxCap := sorted[len(sorted)-1].ItemsPerMin
	for _, belt := range sorted {
		if belt.ItemsPerMin >= ratePerMin {
			return MkRecommendation{
				Mk:             beltMkNumber(belt.ClassName),
				RatePerMin:     ratePerMin,
				CapacityPerMin: belt.ItemsPerMin,
				ExceedsMax:     false,
				Unit:           unit,
			}
		}
	}
	return MkRecommendation{
		Mk:             beltMkNumber(sorted[len(sorted)-1].ClassName),
		RatePerMin:     ratePerMin,
		CapacityPerMin: maxCap,
		ExceedsMax:     true,
		Unit:           unit,
	}
}

// RecommendPipeMk returns the smallest pipe Mk whose capacity covers rate m³/min.
func (c *Catalog) RecommendPipeMk(ratePerMin float64) MkRecommendation {
	unit := "m³/min"
	if len(c.Pipes) == 0 {
		return MkRecommendation{RatePerMin: ratePerMin, Unit: unit, ExceedsMax: ratePerMin > 0}
	}

	sorted := append([]Pipe(nil), c.Pipes...)
	sort.Slice(sorted, func(i, j int) bool {
		return pipeMkNumber(sorted[i].ClassName) < pipeMkNumber(sorted[j].ClassName)
	})

	maxCap := sorted[len(sorted)-1].CubicMetersPerMin
	for _, pipe := range sorted {
		if pipe.CubicMetersPerMin >= ratePerMin {
			return MkRecommendation{
				Mk:             pipeMkNumber(pipe.ClassName),
				RatePerMin:     ratePerMin,
				CapacityPerMin: pipe.CubicMetersPerMin,
				ExceedsMax:     false,
				Unit:           unit,
			}
		}
	}
	return MkRecommendation{
		Mk:             pipeMkNumber(sorted[len(sorted)-1].ClassName),
		RatePerMin:     ratePerMin,
		CapacityPerMin: maxCap,
		ExceedsMax:     true,
		Unit:           unit,
	}
}

// RecommendTransportMk picks belt or pipe guidance based on item form.
func (c *Catalog) RecommendTransportMk(itemClass string, ratePerMin float64) MkRecommendation {
	if item, ok := c.ItemByClass(itemClass); ok && (item.Form == ItemFormLiquid || item.Form == ItemFormGas) {
		return c.RecommendPipeMk(ratePerMin)
	}
	return c.RecommendBeltMk(ratePerMin)
}

// AnalyzeGraph computes node rates, edge flows, imbalance, and total power.
func (c *Catalog) AnalyzeGraph(graph Graph) BalanceResult {
	nodeRates := make(map[string]NodeRates, len(graph.Nodes))
	var totalPower float64

	for _, node := range graph.Nodes {
		rates := NodeRates{
			Outputs: map[string]float64{},
			Inputs:  map[string]float64{},
		}

		switch node.Role {
		case "source":
			if node.ItemClass != "" && node.RatePerMin > 0 {
				rates.Outputs[node.ItemClass] = node.RatePerMin * nodeCount(node.Count)
			}
		case "process", "":
			if node.RecipeClass != "" && node.BuildingClass != "" {
				recipe, ok := c.recipesByClass[node.RecipeClass]
				if ok {
					building, ok := c.BuildingByClass(node.BuildingClass)
					if ok {
						mult := nodeCount(node.Count)
						clock := node.ClockPercent
						if clock <= 0 {
							clock = 100
						}
						sloopMult := SomersloopOutputMultiplier(building, node.SomersloopCount)
						cyclesPerMin := (60 / recipe.DurationSec) * building.ManufacturingSpeed * (clock / 100) * mult * sloopMult
						for _, product := range recipe.Products {
							rates.Outputs[product.ClassName] = cyclesPerMin * product.Amount
						}
						for _, ingredient := range recipe.Ingredients {
							rates.Inputs[ingredient.ClassName] = cyclesPerMin * ingredient.Amount
						}
						power, _ := BuildingPowerMW(c, node.BuildingClass, clock, node.SomersloopCount)
						power *= mult
						rates.PowerMW = power
						totalPower += power
					}
				}
			}
		}
		nodeRates[node.ID] = rates
	}

	edgeFlows := make(map[string]EdgeBalance, len(graph.Edges))

	for _, edge := range graph.Edges {
		src := nodeRates[edge.SourceNodeID]
		dst := nodeRates[edge.TargetNodeID]
		supply := src.Outputs[edge.ItemClass]
		demand := dst.Inputs[edge.ItemClass]

		sameItemOutgoing := 0
		for _, other := range graph.Edges {
			if other.SourceNodeID == edge.SourceNodeID && other.ItemClass == edge.ItemClass {
				sameItemOutgoing++
			}
		}
		available := 0.0
		if sameItemOutgoing > 0 {
			available = supply / float64(sameItemOutgoing)
		}
		flow := available
		if demand > 0 && flow > demand {
			flow = demand
		}
		rec := c.RecommendTransportMk(edge.ItemClass, flow)

		edgeFlows[edge.ID] = EdgeBalance{
			ItemClass:      edge.ItemClass,
			FlowPerMin:     flow,
			DemandPerMin:   demand,
			SupplyPerMin:   supply,
			OverPerMin:     math.Max(0, available-demand),
			UnderPerMin:    math.Max(0, demand-flow),
			RecommendedMk:  rec.Mk,
			CapacityPerMin: rec.CapacityPerMin,
			ExceedsMax:     rec.ExceedsMax,
			Unit:           rec.Unit,
		}
	}

	return BalanceResult{
		Nodes:        nodeRates,
		Edges:        edgeFlows,
		TotalPowerMW: totalPower,
	}
}

func beltMkNumber(className string) int {
	switch {
	case strings.Contains(className, "Mk6"):
		return 6
	case strings.Contains(className, "Mk5"):
		return 5
	case strings.Contains(className, "Mk4"):
		return 4
	case strings.Contains(className, "Mk3"):
		return 3
	case strings.Contains(className, "Mk2"):
		return 2
	default:
		return 1
	}
}

func pipeMkNumber(className string) int {
	if strings.Contains(className, "MK2") || strings.Contains(className, "Mk2") {
		return 2
	}
	return 1
}

// WithinPowerTolerance reports whether two MW values match within wiki test tolerance.
func WithinPowerTolerance(got, want float64) bool {
	got2 := math.Round(got*100) / 100
	want2 := math.Round(want*100) / 100
	if math.Abs(got2-want2) <= powerToleranceMW {
		return true
	}
	// Wiki tables sometimes publish one-decimal MW (e.g. 53.7 for amplified 250% constructor).
	return math.Abs(math.Round(got*10)/10-want) <= powerToleranceMW
}

func nodeCount(count float64) float64 {
	if count <= 0 {
		return 1
	}
	return count
}
