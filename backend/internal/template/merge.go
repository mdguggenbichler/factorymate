package template

import (
	"encoding/json"
	"fmt"
)

// Merge resolves an effective template from defaults and an optional partial override (§5.4).
// Override JSON may contain only "plain", only "embed", or both; absent keys fall back to defaults.
func Merge(defaults Template, overrideJSON []byte) (Template, error) {
	if len(overrideJSON) == 0 {
		return defaults, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(overrideJSON, &raw); err != nil {
		return Template{}, fmt.Errorf("parse template override: %w", err)
	}

	out := defaults
	if rawPlain, ok := raw["plain"]; ok {
		if err := json.Unmarshal(rawPlain, &out.Plain); err != nil {
			return Template{}, fmt.Errorf("parse plain override: %w", err)
		}
	}
	if rawEmbed, ok := raw["embed"]; ok {
		var embed EmbedTemplate
		if err := json.Unmarshal(rawEmbed, &embed); err != nil {
			return Template{}, fmt.Errorf("parse embed override: %w", err)
		}
		out.Embed = &embed
	}
	return out, nil
}

// MergeTemplates merges struct overrides where non-nil/non-empty override fields win.
func MergeTemplates(defaults Template, override *Template) Template {
	if override == nil {
		return defaults
	}
	out := defaults
	if override.Plain != "" {
		out.Plain = override.Plain
	}
	if override.Embed != nil {
		out.Embed = override.Embed
	}
	return out
}
