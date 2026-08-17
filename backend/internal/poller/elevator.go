package poller

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"factorymate/internal/frm"
)

// PhaseMapping is one row of elevator_phases.json (§4.2).
type PhaseMapping struct {
	Phase      int      `json:"phase"`
	Name       string   `json:"name"`
	ClassNames []string `json:"class_names"`
}

// ElevatorPhases holds the reference lookup table loaded from data/elevator_phases.json.
type ElevatorPhases struct {
	Phases []PhaseMapping `json:"phases"`
	byKey  map[string]int // sorted class-name set → phase number
}

// LoadElevatorPhases reads the phase mapping file from path.
func LoadElevatorPhases(path string) (*ElevatorPhases, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read elevator phases %q: %w", path, err)
	}
	var ep ElevatorPhases
	if err := json.Unmarshal(body, &ep); err != nil {
		return nil, fmt.Errorf("parse elevator phases: %w", err)
	}
	ep.buildIndex()
	return &ep, nil
}

func (ep *ElevatorPhases) buildIndex() {
	ep.byKey = make(map[string]int, len(ep.Phases))
	for _, p := range ep.Phases {
		key := classNameSetKey(p.ClassNames)
		ep.byKey[key] = p.Phase
	}
}

// LookupPhase returns the phase number for a CurrentPhase slice, or false if unknown.
func (ep *ElevatorPhases) LookupPhase(items []frm.PhaseItem) (int, bool) {
	if ep == nil || len(ep.byKey) == 0 {
		return 0, false
	}
	names := classNamesFromPhaseItems(items)
	key := classNameSetKey(names)
	phase, ok := ep.byKey[key]
	return phase, ok
}

func classNamesFromPhaseItems(items []frm.PhaseItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		if item.ClassName != "" {
			names = append(names, item.ClassName)
		}
	}
	sort.Strings(names)
	return names
}

func classNameSetKey(classNames []string) string {
	sorted := append([]string(nil), classNames...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
}

// SortedClassNamesJSON returns a JSON array of sorted ClassNames for dedup comparison.
func SortedClassNamesJSON(items []frm.PhaseItem) (string, error) {
	names := classNamesFromPhaseItems(items)
	raw, err := json.Marshal(names)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// DefaultElevatorPhasesPath returns the default path to elevator_phases.json.
func DefaultElevatorPhasesPath() string {
	if path := os.Getenv("ELEVATOR_PHASES_PATH"); path != "" {
		return path
	}
	return "data/elevator_phases.json"
}
