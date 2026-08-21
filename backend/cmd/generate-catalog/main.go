package main

import (
	"log"
	"os"
	"path/filepath"

	"factorymate/internal/planner"
)

// Generate slim UTF-8 catalog from backend/testdata/planner/FactoryGame-Docs.json.
func main() {
	docsPath := envOr("PLANNER_DOCS_PATH", "testdata/planner/FactoryGame-Docs.json")
	iconsJSON := envOr("PLANNER_ICONS_JSON", "../assets/icons.json")
	outPath := envOr("PLANNER_CATALOG_PATH", "testdata/planner/factory_catalog.json")

	cat, err := planner.LoadCatalogFromDocs(docsPath, iconsJSON)
	if err != nil {
		log.Fatalf("load catalog: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	if err := cat.SaveCatalogJSON(outPath); err != nil {
		log.Fatalf("write catalog: %v", err)
	}
	log.Printf("wrote %s (%d items, %d recipes)", outPath, len(cat.Items), len(cat.Recipes))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
