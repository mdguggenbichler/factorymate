package template

// SampleVariables returns §5.4.1 sample data for validation and preview.
func SampleVariables(messageTypeKey string) map[string]string {
	switch messageTypeKey {
	case "server_online", "server_offline":
		return map[string]string{"ServerName": "GuggiRaid Factory"}
	case "player_joined", "player_left":
		return map[string]string{"PlayerName": "Guggi", "OnlineCount": "3"}
	case "fuse_tripped", "power_restored":
		return map[string]string{"CircuitID": "1"}
	case "milestone_unlocked":
		return map[string]string{
			"SchematicName": "Oil Processing",
			"TechTier":      "5",
			"RecipeNames":   "Plastic, Rubber",
		}
	case "hard_drive_ready":
		return map[string]string{
			"SchematicName": "Hard Drive (MAM)",
			"RecipeOptions": "Steel Screw\nCopper Sheet",
		}
	case "elevator_phase_complete":
		return map[string]string{
			"ElevatorName": "Space Elevator",
			"PhaseNumber":  "2",
		}
	case "research_unlocked":
		return map[string]string{
			"NodeName": "Oil Processing",
			"TreeName": "MAM",
			"TechTier": "5",
		}
	case "train_derailed":
		return map[string]string{
			"TrainName":   "Train 1",
			"StationName": "Main Station",
		}
	case "vehicle_out_of_fuel", "vehicle_stuck":
		return map[string]string{
			"VehicleType": "Explorer",
			"VehicleName": "Explorer",
		}
	default:
		return map[string]string{}
	}
}

// AllowedVariables returns the catalog variables for a message type (spec §5.2).
func AllowedVariables(messageTypeKey string) []string {
	switch messageTypeKey {
	case "server_online", "server_offline":
		return []string{"ServerName"}
	case "player_joined", "player_left":
		return []string{"PlayerName", "OnlineCount"}
	case "fuse_tripped", "power_restored":
		return []string{"CircuitID"}
	case "milestone_unlocked":
		return []string{"SchematicName", "TechTier", "RecipeNames"}
	case "hard_drive_ready":
		return []string{"SchematicName", "RecipeOptions"}
	case "elevator_phase_complete":
		return []string{"ElevatorName", "PhaseNumber"}
	case "research_unlocked":
		return []string{"NodeName", "TreeName", "TechTier"}
	case "train_derailed":
		return []string{"TrainName", "StationName"}
	case "vehicle_out_of_fuel", "vehicle_stuck":
		return []string{"VehicleType", "VehicleName"}
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
