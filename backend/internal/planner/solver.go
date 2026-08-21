package planner

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	layoutXSpacing = 220
	layoutYSpacing = 120
)

// Viewport is persisted pan/zoom state for the canvas.
type Viewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

// PlanGraph is the persisted graph_json / baseline_json shape.
type PlanGraph struct {
	Viewport Viewport    `json:"viewport"`
	Nodes    []PlanNode  `json:"nodes"`
	Edges    []PlanEdge  `json:"edges"`
}

// PlanNode is one node on the planner canvas.
type PlanNode struct {
	ID              string  `json:"id"`
	Role            string  `json:"role"`
	RecipeClass     string  `json:"recipeClass,omitempty"`
	BuildingClass   string  `json:"buildingClass,omitempty"`
	ItemClass       string  `json:"itemClass,omitempty"`
	Count           float64 `json:"count,omitempty"`
	ClockPercent    float64 `json:"clockPercent,omitempty"`
	SomersloopCount int     `json:"somersloopCount,omitempty"`
	RatePerMin      float64 `json:"ratePerMin,omitempty"`
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
}

// PlanEdge connects two node ports.
type PlanEdge struct {
	ID           string `json:"id"`
	SourceNodeID string `json:"sourceNodeId"`
	SourcePort   string `json:"sourcePort"`
	TargetNodeID string `json:"targetNodeId"`
	TargetPort   string `json:"targetPort"`
	ItemClass    string `json:"itemClass"`
}

// SolverOptions stores default overrides and recipe choices.
type SolverOptions struct {
	RecipeByProductClass   map[string]string `json:"recipeByProductClass"`
	DefaultClockPercent    float64           `json:"defaultClockPercent"`
	DefaultSomersloopCount int               `json:"defaultSomersloopCount"`
}

// SuggestRequest is the solver input.
type SuggestRequest struct {
	ItemClass              string
	RatePerMin             float64
	RecipeByProductClass   map[string]string
	DefaultClockPercent    float64
	DefaultSomersloopCount int
}

// CycleError is returned when recipe recursion would loop.
type CycleError struct {
	Path []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("production cycle detected: %s", strings.Join(e.Path, " -> "))
}

// EmptyPlanGraph returns the default empty graph for new plans.
func EmptyPlanGraph() PlanGraph {
	return PlanGraph{
		Viewport: Viewport{X: 0, Y: 0, Zoom: 1},
		Nodes:    []PlanNode{},
		Edges:    []PlanEdge{},
	}
}

// Suggest builds a greedy production tree for the requested item and rate.
func Suggest(cat *Catalog, req SuggestRequest) (PlanGraph, error) {
	if cat == nil {
		return PlanGraph{}, errors.New("catalog unavailable")
	}
	if req.ItemClass == "" {
		return PlanGraph{}, errors.New("itemClass is required")
	}
	if req.RatePerMin <= 0 {
		return PlanGraph{}, errors.New("ratePerMin must be positive")
	}
	if req.DefaultClockPercent <= 0 {
		req.DefaultClockPercent = 100
	}
	if req.RecipeByProductClass == nil {
		req.RecipeByProductClass = map[string]string{}
	}

	builder := &suggestBuilder{
		cat:     cat,
		req:     req,
		byClass: buildProducersIndex(cat),
	}
	if _, err := builder.expand(req.ItemClass, req.RatePerMin, nil); err != nil {
		return PlanGraph{}, err
	}
	builder.layout()
	return PlanGraph{
		Viewport: Viewport{X: 0, Y: 0, Zoom: 1},
		Nodes:    builder.nodes,
		Edges:    builder.edges,
	}, nil
}

type suggestBuilder struct {
	cat     *Catalog
	req     SuggestRequest
	byClass map[string][]*Recipe

	nodes      []PlanNode
	edges      []PlanEdge
	nodeMeta   []nodeMeta
	maxDepth   int
	siblingAt  map[int]int
}

type nodeMeta struct {
	depth int
}

func buildProducersIndex(cat *Catalog) map[string][]*Recipe {
	index := make(map[string][]*Recipe)
	for i := range cat.Recipes {
		recipe := &cat.Recipes[i]
		if len(recipe.ProducedIn) == 0 {
			continue
		}
		for _, product := range recipe.Products {
			index[product.ClassName] = append(index[product.ClassName], recipe)
		}
	}
	return index
}

func (b *suggestBuilder) expand(itemClass string, rate float64, visiting []string) (string, error) {
	for _, v := range visiting {
		if v == itemClass {
			path := append(append([]string{}, visiting...), itemClass)
			return "", &CycleError{Path: path}
		}
	}

	if b.isSourceItem(itemClass) {
		id := b.newID("src")
		b.nodes = append(b.nodes, PlanNode{
			ID:         id,
			Role:       "source",
			ItemClass:  itemClass,
			RatePerMin: rate,
		})
		b.nodeMeta = append(b.nodeMeta, nodeMeta{depth: len(visiting)})
		return id, nil
	}

	recipe, buildingClass, err := b.resolveRecipe(itemClass)
	if err != nil {
		return "", err
	}

	outputPerMin, ok := RecipeOutputPerMinute(
		b.cat,
		recipe.ClassName,
		itemClass,
		buildingClass,
		b.req.DefaultClockPercent,
		b.req.DefaultSomersloopCount,
	)
	if !ok || outputPerMin <= 0 {
		return "", fmt.Errorf("cannot compute output for %s", itemClass)
	}

	count := rate / outputPerMin
	processID := b.newID("proc")
	depth := len(visiting)
	b.nodes = append(b.nodes, PlanNode{
		ID:              processID,
		Role:            "process",
		RecipeClass:     recipe.ClassName,
		BuildingClass:   buildingClass,
		Count:           count,
		ClockPercent:    b.req.DefaultClockPercent,
		SomersloopCount: b.req.DefaultSomersloopCount,
	})
	b.nodeMeta = append(b.nodeMeta, nodeMeta{depth: depth})
	if depth > b.maxDepth {
		b.maxDepth = depth
	}

	building, _ := b.cat.BuildingByClass(buildingClass)
	childVisiting := append(append([]string{}, visiting...), itemClass)
	for _, ingredient := range recipe.Ingredients {
		ingredientRate := ingredientRatePerMin(recipe, building, ingredient.ClassName, count, b.req.DefaultClockPercent, b.req.DefaultSomersloopCount)
		childID, err := b.expand(ingredient.ClassName, ingredientRate, childVisiting)
		if err != nil {
			return "", err
		}
		b.edges = append(b.edges, PlanEdge{
			ID:           b.newID("edge"),
			SourceNodeID: childID,
			SourcePort:   "out:" + ingredient.ClassName,
			TargetNodeID: processID,
			TargetPort:   "in:" + ingredient.ClassName,
			ItemClass:    ingredient.ClassName,
		})
	}

	return processID, nil
}

func ingredientRatePerMin(recipe *Recipe, building *Building, itemClass string, machineCount, clock float64, sloops int) float64 {
	if building == nil {
		return 0
	}
	var amount float64
	for _, ing := range recipe.Ingredients {
		if ing.ClassName == itemClass {
			amount = ing.Amount
			break
		}
	}
	if amount <= 0 {
		return 0
	}
	cyclesPerMin := (60 / recipe.DurationSec) * building.ManufacturingSpeed * (clock / 100)
	perMachine := cyclesPerMin * amount * SomersloopOutputMultiplier(building, sloops)
	return machineCount * perMachine
}

func (b *suggestBuilder) byClassRecipe(className string) *Recipe {
	for i := range b.cat.Recipes {
		if b.cat.Recipes[i].ClassName == className {
			return &b.cat.Recipes[i]
		}
	}
	return nil
}

func (b *suggestBuilder) isSourceItem(itemClass string) bool {
	if isRawResourceClass(itemClass) {
		return true
	}
	producers := b.byClass[itemClass]
	if len(producers) == 0 {
		return true
	}
	for _, recipe := range producers {
		if recipeHasAutomatedBuilding(b.cat, recipe) {
			return false
		}
	}
	return true
}

func isRawResourceClass(itemClass string) bool {
	switch {
	case strings.HasPrefix(itemClass, "Desc_Ore"):
		return true
	case itemClass == "Desc_LiquidOil_C", itemClass == "Desc_Water_C", itemClass == "Desc_NitrogenGas_C":
		return true
	default:
		return false
	}
}

func recipeHasAutomatedBuilding(cat *Catalog, recipe *Recipe) bool {
	for _, className := range recipe.ProducedIn {
		if isExtractorBuilding(className) || isExcludedBuilding(className) {
			continue
		}
		if _, ok := cat.BuildingByClass(className); ok {
			return true
		}
	}
	return false
}

func isExcludedBuilding(className string) bool {
	return className == "Build_C" || className == "Build_Converter_C"
}

func isExtractorBuilding(className string) bool {
	return strings.HasPrefix(className, "Build_Miner") ||
		strings.HasPrefix(className, "Build_WaterPump") ||
		strings.HasPrefix(className, "Build_OilPump") ||
		strings.Contains(className, "ResourceExtractor")
}

func (b *suggestBuilder) resolveRecipe(itemClass string) (*Recipe, string, error) {
	if override, ok := b.req.RecipeByProductClass[itemClass]; ok && override != "" {
		recipe := b.byClassRecipe(override)
		if recipe == nil {
			return nil, "", fmt.Errorf("unknown recipe %s", override)
		}
		building, err := b.firstAutomatedBuilding(recipe)
		if err != nil {
			return nil, "", err
		}
		return recipe, building, nil
	}

	candidates := b.byClass[itemClass]
	var nonAlternates []*Recipe
	for _, recipe := range candidates {
		if !recipe.IsAlternate && recipeHasAutomatedBuilding(b.cat, recipe) {
			nonAlternates = append(nonAlternates, recipe)
		}
	}
	if len(nonAlternates) == 1 {
		building, err := b.firstAutomatedBuilding(nonAlternates[0])
		if err != nil {
			return nil, "", err
		}
		return nonAlternates[0], building, nil
	}
	if preferred := b.preferredDefaultRecipe(itemClass, nonAlternates); preferred != nil {
		building, err := b.firstAutomatedBuilding(preferred)
		if err != nil {
			return nil, "", err
		}
		return preferred, building, nil
	}

	for _, recipe := range candidates {
		if recipeHasAutomatedBuilding(b.cat, recipe) {
			building, err := b.firstAutomatedBuilding(recipe)
			if err != nil {
				return nil, "", err
			}
			return recipe, building, nil
		}
	}

	return nil, "", fmt.Errorf("no automated recipe for %s", itemClass)
}

func (b *suggestBuilder) preferredDefaultRecipe(itemClass string, recipes []*Recipe) *Recipe {
	candidates := defaultRecipeClassNames(itemClass)
	for _, want := range candidates {
		for _, recipe := range recipes {
			if recipe.ClassName == want {
				return recipe
			}
		}
	}
	return nil
}

func defaultRecipeClassNames(itemClass string) []string {
	if !strings.HasPrefix(itemClass, "Desc_") || !strings.HasSuffix(itemClass, "_C") {
		return nil
	}
	core := strings.TrimSuffix(strings.TrimPrefix(itemClass, "Desc_"), "_C")
	names := []string{"Recipe_" + core + "_C"}
	if strings.HasSuffix(core, "Ingot") {
		metal := strings.TrimSuffix(core, "Ingot")
		names = append(names, "Recipe_Ingot"+metal+"_C")
	}
	return names
}

func (b *suggestBuilder) firstAutomatedBuilding(recipe *Recipe) (string, error) {
	for _, className := range recipe.ProducedIn {
		if isExtractorBuilding(className) || isExcludedBuilding(className) {
			continue
		}
		if _, ok := b.cat.BuildingByClass(className); ok {
			return className, nil
		}
	}
	return "", fmt.Errorf("no automated building for recipe %s", recipe.ClassName)
}

func (b *suggestBuilder) layout() {
	if b.siblingAt == nil {
		b.siblingAt = make(map[int]int)
	}
	for i := range b.nodes {
		meta := b.nodeMeta[i]
		yIndex := b.siblingAt[meta.depth]
		b.siblingAt[meta.depth] = yIndex + 1
		b.nodes[i].X = float64(b.maxDepth-meta.depth) * layoutXSpacing
		b.nodes[i].Y = float64(yIndex) * layoutYSpacing
	}
}

func (b *suggestBuilder) newID(prefix string) string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return prefix + "_" + hex.EncodeToString(buf[:])
}

// ToBalanceGraph converts a plan graph into the balance package shape.
func ToBalanceGraph(g PlanGraph) Graph {
	nodes := make([]GraphNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes = append(nodes, GraphNode{
			ID:              n.ID,
			Role:            n.Role,
			RecipeClass:     n.RecipeClass,
			BuildingClass:   n.BuildingClass,
			ItemClass:       n.ItemClass,
			Count:           n.Count,
			ClockPercent:    n.ClockPercent,
			SomersloopCount: n.SomersloopCount,
			RatePerMin:      n.RatePerMin,
		})
	}
	edges := make([]GraphEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		edges = append(edges, GraphEdge{
			ID:           e.ID,
			SourceNodeID: e.SourceNodeID,
			SourcePort:   e.SourcePort,
			TargetNodeID: e.TargetNodeID,
			TargetPort:   e.TargetPort,
			ItemClass:    e.ItemClass,
		})
	}
	return Graph{Nodes: nodes, Edges: edges}
}

// RoundCount rounds a machine count to two decimal places for stable JSON.
func RoundCount(v float64) float64 {
	return math.Round(v*100) / 100
}
