package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

type SeedConfig struct {
	FRMHost string
	FRMPort int
}

type messageTypeMeta struct {
	Key       string
	Label     string
	Category  string
	Variables []string
}

var messageTypeCatalog = []messageTypeMeta{
	{Key: "server_online", Label: "Server Online", Category: "server", Variables: []string{"Timestamp", "ServerName", "InGameTime"}},
	{Key: "server_offline", Label: "Server Offline", Category: "server", Variables: []string{"Timestamp", "ServerName", "InGameTime"}},
	{Key: "player_joined", Label: "Player Joined", Category: "player", Variables: []string{"Timestamp", "ServerName", "PlayerName", "OnlineCount"}},
	{Key: "player_left", Label: "Player Left", Category: "player", Variables: []string{"Timestamp", "ServerName", "PlayerName", "OnlineCount"}},
	{Key: "fuse_tripped", Label: "Fuse Tripped", Category: "power", Variables: []string{"Timestamp", "ServerName", "CircuitID", "PowerProduction", "PowerConsumed", "PowerCapacity", "BatteryPercent", "BatteryTimeEmpty"}},
	{Key: "power_restored", Label: "Power Restored", Category: "power", Variables: []string{"Timestamp", "ServerName", "CircuitID", "PowerProduction", "PowerConsumed", "PowerCapacity", "BatteryPercent", "BatteryTimeEmpty"}},
	{Key: "milestone_unlocked", Label: "Milestone Unlocked", Category: "progression", Variables: []string{"Timestamp", "ServerName", "SchematicName", "TechTier", "RecipeNames"}},
	{Key: "hard_drive_ready", Label: "Hard Drive Ready", Category: "progression", Variables: []string{"Timestamp", "ServerName", "SchematicName", "RecipeOptions"}},
	{Key: "elevator_phase_complete", Label: "Elevator Phase Complete", Category: "progression", Variables: []string{"Timestamp", "ServerName", "ElevatorName", "PhaseNumber", "PhaseRequirements"}},
	{Key: "research_unlocked", Label: "Research Unlocked", Category: "progression", Variables: []string{"Timestamp", "ServerName", "NodeName", "TreeName", "TechTier", "ResearchCost"}},
	{Key: "train_derailed", Label: "Train Derailed", Category: "vehicle", Variables: []string{"Timestamp", "ServerName", "TrainName", "StationName", "TrainStatus", "SelfDriving"}},
	{Key: "vehicle_out_of_fuel", Label: "Vehicle Out of Fuel", Category: "vehicle", Variables: []string{"Timestamp", "ServerName", "VehicleType", "VehicleName", "Driver", "ForwardSpeed"}},
	{Key: "vehicle_stuck", Label: "Vehicle Stuck", Category: "vehicle", Variables: []string{"Timestamp", "ServerName", "VehicleType", "VehicleName", "Driver", "ForwardSpeed"}},
}

// Seed inserts idempotent reference data required at startup.
func Seed(ctx context.Context, db *sql.DB, cfg SeedConfig) error {
	if err := seedMessageTypes(ctx, db); err != nil {
		return err
	}
	return seedAppSettings(ctx, db, cfg)
}

func seedMessageTypes(ctx context.Context, db *sql.DB) error {
	defaults, err := loadMessageDefaults()
	if err != nil {
		return err
	}

	for _, meta := range messageTypeCatalog {
		template, ok := defaults[meta.Key]
		if !ok {
			return fmt.Errorf("message_defaults.json missing key %q", meta.Key)
		}

		templateJSON, err := json.Marshal(template)
		if err != nil {
			return fmt.Errorf("marshal template for %q: %w", meta.Key, err)
		}

		variablesJSON, err := json.Marshal(meta.Variables)
		if err != nil {
			return fmt.Errorf("marshal variables for %q: %w", meta.Key, err)
		}

		_, err = db.ExecContext(ctx, `
			INSERT INTO message_types (key, label, category, enabled, default_template_json, variables_json)
			VALUES (?, ?, ?, 1, ?, ?)
			ON CONFLICT(key) DO UPDATE SET
				default_template_json = excluded.default_template_json,
				variables_json = excluded.variables_json`,
			meta.Key, meta.Label, meta.Category, string(templateJSON), string(variablesJSON),
		)
		if err != nil {
			return fmt.Errorf("seed message type %q: %w", meta.Key, err)
		}
	}

	return nil
}

func seedAppSettings(ctx context.Context, db *sql.DB, cfg SeedConfig) error {
	_, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO app_settings (id, server_name, frm_host, frm_port)
		VALUES (1, 'Satisfactory Server', ?, ?)`,
		cfg.FRMHost, cfg.FRMPort,
	)
	if err != nil {
		return fmt.Errorf("seed app_settings: %w", err)
	}
	return nil
}

func loadMessageDefaults() (map[string]json.RawMessage, error) {
	path := messageDefaultsPath()
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read message defaults %q: %w", path, err)
	}

	var defaults map[string]json.RawMessage
	if err := json.Unmarshal(body, &defaults); err != nil {
		return nil, fmt.Errorf("parse message defaults: %w", err)
	}
	return defaults, nil
}

func messageDefaultsPath() string {
	if path := os.Getenv("MESSAGE_DEFAULTS_PATH"); path != "" {
		return path
	}
	return "data/message_defaults.json"
}

// SeedConfigFromEnv builds seed configuration from environment variables (spec §9).
func SeedConfigFromEnv() SeedConfig {
	host := os.Getenv("FRM_HOST")
	port := 8080
	if raw := os.Getenv("FRM_PORT"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			port = parsed
		}
	}
	return SeedConfig{FRMHost: host, FRMPort: port}
}

// Init runs migrations and seed in order.
func Init(ctx context.Context, db *sql.DB, cfg SeedConfig) error {
	if err := Migrate(ctx, db); err != nil {
		return err
	}
	return Seed(ctx, db, cfg)
}
