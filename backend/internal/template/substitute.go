package template

import (
	"regexp"
	"sort"
)

var varPattern = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9]*)\}`)

// Substitute replaces {VarName} placeholders with values from vars (spec §5.4).
// Missing keys render as empty strings.
func Substitute(s string, vars map[string]string) string {
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := match[1 : len(match)-1]
		if v, ok := vars[name]; ok {
			return v
		}
		return ""
	})
}

// ExtractVariables returns sorted unique variable names referenced in a template.
func ExtractVariables(tmpl Template) []string {
	seen := make(map[string]struct{})
	add := func(s string) {
		for _, name := range varPattern.FindAllStringSubmatch(s, -1) {
			seen[name[1]] = struct{}{}
		}
	}

	add(tmpl.Plain)
	if tmpl.Embed != nil {
		add(tmpl.Embed.Title)
		add(tmpl.Embed.Description)
		add(tmpl.Embed.Color)
		for _, f := range tmpl.Embed.Fields {
			add(f.Name)
			add(f.Value)
		}
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
