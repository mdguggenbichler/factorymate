package template

// SampleVariables returns §5.4.1 sample data for validation and preview.
func SampleVariables(messageTypeKey string) map[string]string {
	switch messageTypeKey {
	case "server_online", "server_offline":
		return withSystemVars(map[string]string{
			"ServerName": "CBC | Conveyor Belt Cult",
			"InGameTime": "Day 42, 14:37",
		})
	case "player_joined", "player_left":
		return withSystemVars(map[string]string{
			"PlayerName":  "Michael",
			"OnlineCount": "4",
		})
	case "fuse_tripped", "power_restored":
		return withSystemVars(map[string]string{
			"CircuitID":         "1",
			"PowerProduction":   "120",
			"PowerConsumed":     "95",
			"PowerCapacity":     "100",
			"BatteryPercent":    "68",
			"BatteryTimeEmpty":  "2h 15m",
		})
	case "milestone_unlocked":
		return withSystemVars(map[string]string{
			"SchematicName": "Oil Processing",
			"TechTier":      "5",
			"RecipeNames":   "Plastic, Rubber",
		})
	case "hub_tier_complete":
		return withSystemVars(map[string]string{
			"TechTier":       "6",
			"MilestoneNames": "Industrial Manufacturing\nMonorail Train Technology\nPipeline Engineering Mk.2",
			"MilestoneCount": "5",
		})
	case "hard_drive_ready":
		return withSystemVars(map[string]string{
			"SchematicName": "Hard Drive (MAM)",
			"RecipeOptions": "Steel Screw\nCopper Sheet",
		})
	case "elevator_phase_complete":
		return withSystemVars(map[string]string{
			"ElevatorName":      "Space Elevator",
			"PhaseNumber":       "2",
			"PhaseRequirements": "Smart Plating: 1000/1000\nVersatile Framework: 500/500\nAutomated Wiring: 100/100",
		})
	case "elevator_phase_done":
		return withSystemVars(map[string]string{
			"ElevatorName":      "Space Elevator",
			"PhaseNumber":       "2",
			"PhaseRequirements": "Smart Plating: 1000/1000\nVersatile Framework: 500/500\nAutomated Wiring: 100/100",
		})
	case "research_unlocked":
		return withSystemVars(map[string]string{
			"NodeName":     "Oil Processing",
			"TreeName":     "MAM",
			"TechTier":     "5",
			"ResearchCost": "Copper Sheet × 10\nCable × 15",
		})
	case "train_derailed":
		return withSystemVars(map[string]string{
			"TrainName":   "Train 1",
			"StationName": "Main Station",
			"TrainStatus": "Self-Driving",
			"SelfDriving": "No error",
		})
	case "vehicle_out_of_fuel", "vehicle_stuck":
		return withSystemVars(map[string]string{
			"VehicleType":  "Explorer",
			"VehicleName":  "Tractor",
			"Driver":       "Michael",
			"ForwardSpeed": "0.0 km/h",
		})
	case "connection_details_changed", "connection_details":
		return withSystemVars(map[string]string{
			"GameHost": "factory.example.com",
			"GamePort": "7777",
			"Notes":    "Use Epic login",
		})
	default:
		return withSystemVars(map[string]string{})
	}
}

func withSystemVars(vars map[string]string) map[string]string {
	out := make(map[string]string, len(vars)+3)
	for k, v := range vars {
		out[k] = v
	}
	if _, ok := out["ServerName"]; !ok {
		out["ServerName"] = "CBC | Conveyor Belt Cult"
	}
	out["Timestamp"] = "Aug 17, 2026 · 14:37 UTC"
	out["TimestampISO"] = "2026-08-17T14:37:00Z"
	return out
}

// AllowedVariables returns the catalog variables for a message type (spec §5.2).
func AllowedVariables(messageTypeKey string) []string {
	system := []string{"Timestamp", "ServerName"}
	switch messageTypeKey {
	case "server_online", "server_offline":
		return append(system, "InGameTime")
	case "player_joined", "player_left":
		return append(system, "PlayerName", "OnlineCount")
	case "fuse_tripped", "power_restored":
		return append(system,
			"CircuitID", "PowerProduction", "PowerConsumed", "PowerCapacity",
			"BatteryPercent", "BatteryTimeEmpty",
		)
	case "milestone_unlocked":
		return append(system, "SchematicName", "TechTier", "RecipeNames")
	case "hub_tier_complete":
		return append(system, "TechTier", "MilestoneNames", "MilestoneCount")
	case "hard_drive_ready":
		return append(system, "SchematicName", "RecipeOptions")
	case "elevator_phase_complete", "elevator_phase_done":
		return append(system, "ElevatorName", "PhaseNumber", "PhaseRequirements")
	case "research_unlocked":
		return append(system, "NodeName", "TreeName", "TechTier", "ResearchCost")
	case "train_derailed":
		return append(system, "TrainName", "StationName", "TrainStatus", "SelfDriving")
	case "vehicle_out_of_fuel", "vehicle_stuck":
		return append(system, "VehicleType", "VehicleName", "Driver", "ForwardSpeed")
	case "connection_details_changed", "connection_details":
		return append(system, "GameHost", "GamePort", "Notes")
	default:
		return nil
	}
}

// ValidateMessageType validates a template using §5.4.1 samples for the given type.
func ValidateMessageType(messageTypeKey string, tmpl Template) error {
	if err := ValidateShape(tmpl); err != nil {
		return err
	}
	return Validate(tmpl, AllowedVariables(messageTypeKey), SampleVariables(messageTypeKey))
}
