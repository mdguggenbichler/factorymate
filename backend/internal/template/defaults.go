package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadDefaults reads message_defaults.json (spec §5.5).
func LoadDefaults() (map[string]Template, error) {
	path := defaultsPath()
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read message defaults %q: %w", path, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse message defaults: %w", err)
	}

	out := make(map[string]Template, len(raw))
	for key, tmplJSON := range raw {
		var tmpl Template
		if err := json.Unmarshal(tmplJSON, &tmpl); err != nil {
			return nil, fmt.Errorf("parse template for %q: %w", key, err)
		}
		out[key] = tmpl
	}
	return out, nil
}

func defaultsPath() string {
	if path := os.Getenv("MESSAGE_DEFAULTS_PATH"); path != "" {
		return path
	}
	if root := findModuleRoot(); root != "" {
		return filepath.Join(root, "data", "message_defaults.json")
	}
	return "data/message_defaults.json"
}

func findModuleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
