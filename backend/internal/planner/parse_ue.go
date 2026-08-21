package planner

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ueItemEntryRe = regexp.MustCompile(`ItemClass="([^"]+)"\s*,\s*Amount=([0-9.]+)`)
	ueClassNameRe = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_]*)_C`)
)

// ItemAmount is a parsed ingredient or product entry.
type ItemAmount struct {
	ClassName string  `json:"className"`
	Amount    float64 `json:"amount"`
}

// ParseUEItemList parses Unreal strings like
// ((ItemClass=".../Desc_IronIngot.Desc_IronIngot_C",Amount=3)).
func ParseUEItemList(raw string) ([]ItemAmount, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "()" {
		return nil, nil
	}

	matches := ueItemEntryRe.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("parse UE item list: no entries in %q", raw)
	}

	out := make([]ItemAmount, 0, len(matches))
	for _, m := range matches {
		amount, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return nil, fmt.Errorf("parse UE item amount %q: %w", m[2], err)
		}
		className, ok := ExtractClassName(m[1])
		if !ok {
			return nil, fmt.Errorf("parse UE item class from %q", m[1])
		}
		out = append(out, ItemAmount{
			ClassName: className,
			Amount:    amount,
		})
	}
	return out, nil
}

// ParseUEClassList parses parenthesized Unreal class path lists such as mProducedIn.
func ParseUEClassList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "()" {
		return nil
	}

	seen := make(map[string]struct{})
	var out []string
	for _, part := range strings.Split(raw, "\"") {
		part = strings.TrimSpace(part)
		if part == "" || part == "(" || part == ")" || part == "," {
			continue
		}
		className, ok := ExtractClassName(part)
		if !ok {
			continue
		}
		if _, exists := seen[className]; exists {
			continue
		}
		seen[className] = struct{}{}
		out = append(out, className)
	}
	return out
}

// ExtractClassName returns the trailing Desc_*_C or Build_*_C ClassName from an Unreal path.
func ExtractClassName(path string) (string, bool) {
	matches := ueClassNameRe.FindAllStringSubmatch(path, -1)
	if len(matches) == 0 {
		return "", false
	}
	last := matches[len(matches)-1][1]
	return last + "_C", true
}
