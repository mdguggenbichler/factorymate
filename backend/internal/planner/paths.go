package planner

import (
	"os"
	"path/filepath"
)

const (
	envDocsPath     = "PLANNER_DOCS_PATH"
	envCatalogPath  = "PLANNER_CATALOG_PATH"
	envIconsDir     = "PLANNER_ICONS_DIR"
	envIconsJSON    = "PLANNER_ICONS_JSON"
)

// Config holds planner data file locations.
type Config struct {
	DocsPath     string
	CatalogPath  string
	IconsDir     string
	IconsJSON    string
}

// DefaultConfig resolves env overrides with local-dev fallbacks.
func DefaultConfig() Config {
	return Config{
		DocsPath:    resolveExistingPath(envDocsPath, "../docs/FactoryGame-Docs.json", "docs/FactoryGame-Docs.json"),
		CatalogPath: resolveExistingPath(envCatalogPath, "data/factory_catalog.json", "testdata/planner/factory_catalog.json", "../backend/data/factory_catalog.json", "backend/testdata/planner/factory_catalog.json"),
		IconsDir:    resolveExistingPath(envIconsDir, "../assets/icons", "assets/icons"),
		IconsJSON:   resolveExistingPath(envIconsJSON, "../assets/icons.json", "assets/icons.json"),
	}
}

func resolveExistingPath(envKey string, candidates ...string) string {
	if p := os.Getenv(envKey); p != "" {
		return p
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

// IconPNGPath returns the on-disk PNG path for a resolved icon ClassName.
func (c Config) IconPNGPath(className string) string {
	return filepath.Join(c.IconsDir, className+".png")
}
