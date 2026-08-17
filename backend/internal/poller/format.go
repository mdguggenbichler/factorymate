package poller

import (
	"fmt"
	"strings"

	"factorymate/internal/frm"
)

type sessionSnapshot struct {
	ServerName string
	InGameTime string
}

func formatInGameTime(info frm.SessionInfo) string {
	return fmt.Sprintf("Day %d, %02d:%02d", info.PassedDays, info.Hours, info.Minutes)
}

func formatMW(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

func formatBatteryPercent(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

func formatForwardSpeed(speed float64) string {
	return fmt.Sprintf("%.1f km/h", speed)
}

func formatDriver(driver string) string {
	if strings.TrimSpace(driver) == "" {
		return "—"
	}
	return driver
}

func powerEventVars(c frm.Circuit, serverName string) map[string]string {
	vars := map[string]string{
		"CircuitID":       intToString(c.CircuitGroupID),
		"ServerName":      serverName,
		"PowerProduction": formatMW(c.PowerProduction),
		"PowerConsumed":   formatMW(c.PowerConsumed),
		"PowerCapacity":   formatMW(c.PowerCapacity),
	}
	if c.BatteryCapacity > 0 {
		vars["BatteryPercent"] = formatBatteryPercent(c.BatteryPercent)
		vars["BatteryTimeEmpty"] = c.BatteryTimeEmpty
	}
	return vars
}

func formatResearchCost(items []frm.Item) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s × %d", item.Name, item.Amount))
	}
	return strings.Join(lines, "\n")
}

func formatPhaseRequirements(items []frm.PhaseItem) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s: %d/%d", item.Name, item.RemainingCost, item.TotalCost))
	}
	return strings.Join(lines, "\n")
}

func humanizeTrainError(code string) string {
	if label, ok := trainErrorLabels[code]; ok {
		return label
	}
	if strings.TrimSpace(code) == "" {
		return "—"
	}
	return code
}

var trainErrorLabels = map[string]string{
	"SDLE_NoError":                      "No error",
	"SDLE_NoPower":                      "No power",
	"SDLE_NoTimeTable":                  "No time table",
	"SDLE_InvalidNextStop":              "Invalid next stop",
	"SDLE_InvalidLocomotivePlacement":   "Invalid locomotive placement",
	"SDLE_NoPath":                       "No path",
	"SDLE_StationUnreachable":           "Station unreachable",
	"SDLE_StationUnreachableWithSignals": "Station unreachable (signals)",
	"SDLE_LongWaitAtSignal":             "Long wait at signal",
	"PDE_NoError":                       "No error",
	"PDE_StationUnreachable":            "Station unreachable",
	"PDE_StationUnreachableWithSignals": "Station unreachable (signals)",
}
