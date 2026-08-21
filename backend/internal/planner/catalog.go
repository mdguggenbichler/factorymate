package planner

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const smelterClassName = "Build_SmelterMk1_C"

var smelterSomersloopOverride = map[string]int{
	smelterClassName: 1,
}

var excludedProducedInSubstrings = []string{
	"WorkBench",
	"Workshop",
	"BuildGun",
	"AutomatedWorkBench",
}

// ItemForm describes how an item moves on belts vs pipes.
type ItemForm string

const (
	ItemFormSolid  ItemForm = "solid"
	ItemFormLiquid ItemForm = "liquid"
	ItemFormGas    ItemForm = "gas"
)

// Catalog is the in-memory slim game-data catalog for the planner.
type Catalog struct {
	Items     []Item     `json:"items"`
	Recipes   []Recipe   `json:"recipes"`
	Buildings []Building `json:"buildings"`
	Belts     []Belt     `json:"belts"`
	Pipes     []Pipe     `json:"pipes"`
	IconMap   IconMap    `json:"iconMap"`

	itemsByClass     map[string]*Item
	recipesByClass   map[string]*Recipe
	buildingsByClass map[string]*Building
	beltsByClass     map[string]*Belt
	pipesByClass     map[string]*Pipe
	iconPaths        map[string]string
}

// Item is a planner item or fluid descriptor.
type Item struct {
	ClassName   string   `json:"className"`
	DisplayName string   `json:"displayName"`
	Form        ItemForm `json:"form"`
}

// Recipe is an automated factory recipe.
type Recipe struct {
	ClassName   string       `json:"className"`
	DisplayName string       `json:"displayName"`
	DurationSec float64      `json:"durationSec"`
	Ingredients []ItemAmount `json:"ingredients"`
	Products    []ItemAmount `json:"products"`
	ProducedIn  []string     `json:"producedIn"`
	IsAlternate bool         `json:"isAlternate"`
}

// Building is a manufacturer, extractor, or pump.
type Building struct {
	ClassName                   string  `json:"className"`
	DisplayName                 string  `json:"displayName"`
	IconClassName               string  `json:"iconClassName"`
	PowerBaseMW                 float64 `json:"powerBaseMW"`
	PowerExponent               float64 `json:"powerExponent"`
	ManufacturingSpeed          float64 `json:"manufacturingSpeed"`
	SomersloopSlots             int     `json:"somersloopSlots"`
	SomersloopBoostMultiplier   float64 `json:"somersloopBoostMultiplier"`
	CanChangeProductionBoost    bool    `json:"canChangeProductionBoost"`
}

// Belt is a conveyor belt or lift Mk table entry.
type Belt struct {
	ClassName   string  `json:"className"`
	DisplayName string  `json:"displayName"`
	ItemsPerMin float64 `json:"itemsPerMin"`
}

// Pipe is a pipeline Mk table entry.
type Pipe struct {
	ClassName        string  `json:"className"`
	DisplayName      string  `json:"displayName"`
	CubicMetersPerMin float64 `json:"cubicMetersPerMin"`
}

// IconMap maps Build_* ClassNames to Desc_* icon ClassNames.
type IconMap map[string]string

type docGroup struct {
	NativeClass string              `json:"NativeClass"`
	Classes     []map[string]json.RawMessage `json:"Classes"`
}

func classString(cls map[string]json.RawMessage, key string) string {
	raw, ok := cls[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

// LoadCatalog loads the slim catalog file when present, otherwise parses the UTF-16 dump.
func LoadCatalog(cfg Config) (*Catalog, error) {
	if cfg.CatalogPath != "" {
		if _, err := os.Stat(cfg.CatalogPath); err == nil {
			return loadCatalogFile(cfg.CatalogPath)
		}
	}
	return parseDocsDump(cfg)
}

// LoadCatalogFromDocs parses the UTF-16 LE BOM dump at path (tests / generator).
func LoadCatalogFromDocs(docsPath, iconsJSONPath string) (*Catalog, error) {
	cfg := Config{
		DocsPath:  docsPath,
		IconsJSON: iconsJSONPath,
	}
	return parseDocsDump(cfg)
}

func loadCatalogFile(path string) (*Catalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cat Catalog
	if err := json.NewDecoder(f).Decode(&cat); err != nil {
		return nil, fmt.Errorf("decode catalog %s: %w", path, err)
	}
	cat.buildIndexes(false)
	return &cat, nil
}

func parseDocsDump(cfg Config) (*Catalog, error) {
	groups, err := decodeDocsJSON(cfg.DocsPath)
	if err != nil {
		return nil, err
	}

	iconPaths, err := loadIconPaths(cfg.IconsJSON)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	items := make(map[string]Item)
	var recipes []Recipe
	var buildings []Building
	var belts []Belt
	var pipes []Pipe
	iconMap := make(IconMap)

	for _, group := range groups {
		native := group.NativeClass
		switch {
		case strings.HasSuffix(native, "FGRecipe'"):
			for _, cls := range group.Classes {
				recipe, err := parseRecipe(cls)
				if err != nil {
					return nil, err
				}
				if recipe == nil {
					continue
				}
				recipes = append(recipes, *recipe)
			}
		case isItemNative(native):
			for _, cls := range group.Classes {
				item, ok := parseItem(cls)
				if !ok {
					continue
				}
				items[item.ClassName] = item
			}
		case strings.HasSuffix(native, "FGBuildableManufacturer'"),
			strings.HasSuffix(native, "FGBuildableManufacturerVariablePower'"):
			for _, cls := range group.Classes {
				building, ok := parseBuilding(cls, iconPaths, iconMap)
				if !ok {
					continue
				}
				buildings = append(buildings, building)
			}
		case strings.HasSuffix(native, "FGBuildableResourceExtractor'"),
			strings.HasSuffix(native, "FGBuildableWaterPump'"):
			for _, cls := range group.Classes {
				building, ok := parseBuilding(cls, iconPaths, iconMap)
				if !ok {
					continue
				}
				buildings = append(buildings, building)
			}
		case strings.HasSuffix(native, "FGBuildableConveyorBelt'"),
			strings.HasSuffix(native, "FGBuildableConveyorLift'"):
			for _, cls := range group.Classes {
				belt, ok := parseBelt(cls)
				if !ok {
					continue
				}
				belts = append(belts, belt)
			}
		case strings.HasSuffix(native, "FGBuildablePipeline'"):
			for _, cls := range group.Classes {
				pipe, ok := parsePipe(cls)
				if !ok {
					continue
				}
				pipes = append(pipes, pipe)
			}
		}
	}

	applySomersloopOverrides(buildings)
	sortItems := sortedItems(items)
	sortRecipes(recipes)
	sortBuildings(buildings)
	sortBelts(belts)
	sortPipes(pipes)

	cat := &Catalog{
		Items:     sortItems,
		Recipes:   recipes,
		Buildings: buildings,
		Belts:     belts,
		Pipes:     pipes,
		IconMap:   iconMap,
	}
	cat.buildIndexes(true)
	cat.iconPaths = iconPaths
	return cat, nil
}

func decodeDocsJSON(path string) ([]docGroup, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open docs dump %s: %w", path, err)
	}
	defer f.Close()

	dec := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	var groups []docGroup
	if err := json.NewDecoder(transform.NewReader(f, dec)).Decode(&groups); err != nil {
		return nil, fmt.Errorf("decode docs dump %s: %w", path, err)
	}
	return groups, nil
}

func isItemNative(native string) bool {
	switch {
	case strings.HasSuffix(native, "FGItemDescriptor'"),
		strings.HasSuffix(native, "FGResourceDescriptor'"),
		strings.HasSuffix(native, "FGItemDescriptorBiomass'"),
		strings.HasSuffix(native, "FGItemDescriptorNuclearFuel'"),
		strings.HasSuffix(native, "FGItemDescriptorPowerBoosterFuel'"):
		return true
	default:
		return false
	}
}

func parseRecipe(cls map[string]json.RawMessage) (*Recipe, error) {
	className := classString(cls, "ClassName")
	if className == "" {
		return nil, nil
	}

	ingredients, err := ParseUEItemList(classString(cls, "mIngredients"))
	if err != nil {
		return nil, fmt.Errorf("recipe %s ingredients: %w", className, err)
	}
	products, err := ParseUEItemList(classString(cls, "mProduct"))
	if err != nil {
		return nil, fmt.Errorf("recipe %s products: %w", className, err)
	}

	producedIn := filterProducedIn(ParseUEClassList(classString(cls, "mProducedIn")))
	if len(producedIn) == 0 {
		return nil, nil
	}

	duration, _ := parseDumpFloat(classString(cls, "mManufactoringDuration"))
	if duration <= 0 {
		return nil, nil
	}

	fullName := classString(cls, "FullName")
	isAlternate := strings.Contains(fullName, "/AlternateRecipes/") ||
		strings.HasPrefix(className, "Recipe_Alternate_")

	displayName := classString(cls, "mDisplayName")
	if displayName == "" {
		displayName = className
	}

	return &Recipe{
		ClassName:   className,
		DisplayName: displayName,
		DurationSec: duration,
		Ingredients: ingredients,
		Products:    products,
		ProducedIn:  producedIn,
		IsAlternate: isAlternate,
	}, nil
}

func parseItem(cls map[string]json.RawMessage) (Item, bool) {
	className := classString(cls, "ClassName")
	if className == "" {
		return Item{}, false
	}
	displayName := classString(cls, "mDisplayName")
	if displayName == "" {
		displayName = className
	}
	return Item{
		ClassName:   className,
		DisplayName: displayName,
		Form:        formFromDump(classString(cls, "mForm")),
	}, true
}

func parseBuilding(cls map[string]json.RawMessage, iconPaths map[string]string, iconMap IconMap) (Building, bool) {
	className := classString(cls, "ClassName")
	if className == "" {
		return Building{}, false
	}
	displayName := classString(cls, "mDisplayName")
	if displayName == "" {
		displayName = className
	}

	powerBase, _ := parseDumpFloat(classString(cls, "mPowerConsumption"))
	powerExp, _ := parseDumpFloat(classString(cls, "mPowerConsumptionExponent"))
	if powerExp == 0 {
		powerExp = 1
	}
	speed, _ := parseDumpFloat(classString(cls, "mManufacturingSpeed"))
	if speed == 0 {
		speed = 1
	}

	slots, _ := parseDumpInt(classString(cls, "mProductionShardSlotSize"))
	boostMult, _ := parseDumpFloat(classString(cls, "mProductionShardBoostMultiplier"))
	canBoost := parseDumpBool(classString(cls, "mCanChangeProductionBoost"))

	iconClass := resolveIconClassName(className, iconPaths, iconMap)

	return Building{
		ClassName:                 className,
		DisplayName:               displayName,
		IconClassName:             iconClass,
		PowerBaseMW:               powerBase,
		PowerExponent:             powerExp,
		ManufacturingSpeed:        speed,
		SomersloopSlots:           slots,
		SomersloopBoostMultiplier: boostMult,
		CanChangeProductionBoost:  canBoost,
	}, true
}

func parseBelt(cls map[string]json.RawMessage) (Belt, bool) {
	className := classString(cls, "ClassName")
	speed, ok := parseDumpFloat(classString(cls, "mSpeed"))
	if className == "" || !ok || speed <= 0 {
		return Belt{}, false
	}
	displayName := classString(cls, "mDisplayName")
	if displayName == "" {
		displayName = className
	}
	return Belt{
		ClassName:   className,
		DisplayName: displayName,
		ItemsPerMin: speed / 2,
	}, true
}

func parsePipe(cls map[string]json.RawMessage) (Pipe, bool) {
	className := classString(cls, "ClassName")
	flow, ok := parseDumpFloat(classString(cls, "mFlowLimit"))
	if className == "" || !ok || flow <= 0 {
		return Pipe{}, false
	}
	displayName := classString(cls, "mDisplayName")
	if displayName == "" {
		displayName = className
	}
	return Pipe{
		ClassName:         className,
		DisplayName:       displayName,
		CubicMetersPerMin: flow * 60,
	}, true
}

func filterProducedIn(classes []string) []string {
	var out []string
	for _, className := range classes {
		if isExcludedProducedIn(className) {
			continue
		}
		out = append(out, className)
	}
	return out
}

func isExcludedProducedIn(className string) bool {
	for _, needle := range excludedProducedInSubstrings {
		if strings.Contains(className, needle) {
			return true
		}
	}
	return false
}

func applySomersloopOverrides(buildings []Building) {
	for i := range buildings {
		if slots, ok := smelterSomersloopOverride[buildings[i].ClassName]; ok {
			if buildings[i].CanChangeProductionBoost {
				buildings[i].SomersloopSlots = slots
			}
		}
	}
}

func resolveIconClassName(buildClass string, iconPaths map[string]string, iconMap IconMap) string {
	if iconClass, ok := iconMap[buildClass]; ok {
		return iconClass
	}
	if strings.HasPrefix(buildClass, "Build_") {
		descClass := "Desc_" + strings.TrimPrefix(buildClass, "Build_")
		if iconPaths != nil {
			if _, ok := iconPaths[descClass]; ok {
				iconMap[buildClass] = descClass
				return descClass
			}
		}
		iconMap[buildClass] = descClass
		return descClass
	}
	iconMap[buildClass] = buildClass
	return buildClass
}

func loadIconPaths(path string) (map[string]string, error) {
	if path == "" {
		return map[string]string{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	raw := map[string]struct {
		FSPath string `json:"fs_path"`
	}{}
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode icons.json: %w", err)
	}
	out := make(map[string]string, len(raw))
	for className := range raw {
		out[className] = className
	}
	return out, nil
}

func formFromDump(raw string) ItemForm {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "RF_LIQUID":
		return ItemFormLiquid
	case "RF_GAS":
		return ItemFormGas
	default:
		return ItemFormSolid
	}
}

func scaleItemAmount(item *ItemAmount, form ItemForm) {
	if form == ItemFormLiquid || form == ItemFormGas {
		item.Amount /= 1000
	}
}

func (c *Catalog) buildIndexes(scaleFluids bool) {
	c.itemsByClass = make(map[string]*Item, len(c.Items))
	for i := range c.Items {
		c.itemsByClass[c.Items[i].ClassName] = &c.Items[i]
	}

	if scaleFluids {
		for i := range c.Recipes {
			for j := range c.Recipes[i].Ingredients {
				if item := c.itemsByClass[c.Recipes[i].Ingredients[j].ClassName]; item != nil {
					scaleItemAmount(&c.Recipes[i].Ingredients[j], item.Form)
				}
			}
			for j := range c.Recipes[i].Products {
				if item := c.itemsByClass[c.Recipes[i].Products[j].ClassName]; item != nil {
					scaleItemAmount(&c.Recipes[i].Products[j], item.Form)
				}
			}
		}
	}

	c.recipesByClass = make(map[string]*Recipe, len(c.Recipes))
	for i := range c.Recipes {
		c.recipesByClass[c.Recipes[i].ClassName] = &c.Recipes[i]
	}
	c.buildingsByClass = make(map[string]*Building, len(c.Buildings))
	for i := range c.Buildings {
		c.buildingsByClass[c.Buildings[i].ClassName] = &c.Buildings[i]
	}
	c.beltsByClass = make(map[string]*Belt, len(c.Belts))
	for i := range c.Belts {
		c.beltsByClass[c.Belts[i].ClassName] = &c.Belts[i]
	}
	c.pipesByClass = make(map[string]*Pipe, len(c.Pipes))
	for i := range c.Pipes {
		c.pipesByClass[c.Pipes[i].ClassName] = &c.Pipes[i]
	}
}

// ItemByClass returns an item descriptor by ClassName.
func (c *Catalog) ItemByClass(className string) (*Item, bool) {
	item, ok := c.itemsByClass[className]
	return item, ok
}

// BuildingByClass returns a building by ClassName.
func (c *Catalog) BuildingByClass(className string) (*Building, bool) {
	b, ok := c.buildingsByClass[className]
	return b, ok
}

// ResolveIconClassName maps a ClassName to the icon ClassName used on disk.
func (c *Catalog) ResolveIconClassName(className string) string {
	if iconClass, ok := c.IconMap[className]; ok && iconClass != "" {
		return iconClass
	}
	if strings.HasPrefix(className, "Build_") {
		return "Desc_" + strings.TrimPrefix(className, "Build_")
	}
	return className
}

// WriteCatalogJSON writes the slim catalog to w.
func (c *Catalog) WriteCatalogJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

// SaveCatalogJSON writes the slim catalog to path.
func (c *Catalog) SaveCatalogJSON(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return c.WriteCatalogJSON(f)
}

func parseDumpFloat(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "none") {
		return 0, false
	}
	v, err := strconvParseFloat(raw)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseDumpInt(raw string) (int, bool) {
	v, ok := parseDumpFloat(raw)
	if !ok {
		return 0, false
	}
	return int(v), true
}

func parseDumpBool(raw string) bool {
	return strings.EqualFold(strings.TrimSpace(raw), "true")
}

func sortedItems(items map[string]Item) []Item {
	out := make([]Item, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ClassName < out[j].ClassName
	})
	return out
}

func sortRecipes(recipes []Recipe) {
	sort.Slice(recipes, func(i, j int) bool {
		return recipes[i].ClassName < recipes[j].ClassName
	})
}

func sortBuildings(buildings []Building) {
	sort.Slice(buildings, func(i, j int) bool {
		return buildings[i].ClassName < buildings[j].ClassName
	})
}

func sortBelts(belts []Belt) {
	sort.Slice(belts, func(i, j int) bool {
		return belts[i].ItemsPerMin < belts[j].ItemsPerMin
	})
}

func sortPipes(pipes []Pipe) {
	sort.Slice(pipes, func(i, j int) bool {
		return pipes[i].CubicMetersPerMin < pipes[j].CubicMetersPerMin
	})
}

// strconvParseFloat avoids importing strconv at top level twice — use strconv.
func strconvParseFloat(raw string) (float64, error) {
	return strconv.ParseFloat(raw, 64)
}
