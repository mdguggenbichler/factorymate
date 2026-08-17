package poller

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"factorymate/internal/frm"
)

func upsertProdStatsState(ctx context.Context, db *sql.DB, stats []frm.ProdStat, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	for _, s := range stats {
		_, err := db.ExecContext(ctx, `
			INSERT INTO prod_stats_state (
				item_class_name, item_display_name, prod_per_min_label, prod_percent, cons_percent,
				current_prod, max_prod, current_consumed, max_consumed, transfer_type, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(item_class_name) DO UPDATE SET
				item_display_name = excluded.item_display_name,
				prod_per_min_label = excluded.prod_per_min_label,
				prod_percent = excluded.prod_percent,
				cons_percent = excluded.cons_percent,
				current_prod = excluded.current_prod,
				max_prod = excluded.max_prod,
				current_consumed = excluded.current_consumed,
				max_consumed = excluded.max_consumed,
				transfer_type = excluded.transfer_type,
				updated_at = excluded.updated_at`,
			s.ClassName, s.Name, s.ProdPerMin, s.ProdPercent, s.ConsPercent,
			s.CurrentProd, s.MaxProd, s.CurrentConsumed, s.MaxConsumed, s.Type, ts,
		)
		if err != nil {
			return fmt.Errorf("upsert prod_stats_state %s: %w", s.ClassName, err)
		}
	}
	return nil
}

func insertProductionSnapshots(ctx context.Context, db *sql.DB, stats []frm.ProdStat, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	for _, s := range stats {
		_, err := db.ExecContext(ctx, `
			INSERT INTO production_snapshots (
				item_class_name, item_display_name, produced_per_min, consumed_per_min, captured_at
			) VALUES (?, ?, ?, ?, ?)`,
			s.ClassName, s.Name, s.CurrentProd, s.CurrentConsumed, ts,
		)
		if err != nil {
			return fmt.Errorf("insert production_snapshots %s: %w", s.ClassName, err)
		}
	}
	return nil
}

func upsertResourceSinkState(ctx context.Context, db *sql.DB, sink frm.ResourceSink, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
		INSERT INTO resource_sink_state (id, num_coupon, percent, points_to_coupon, total_points, updated_at)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			num_coupon = excluded.num_coupon,
			percent = excluded.percent,
			points_to_coupon = excluded.points_to_coupon,
			total_points = excluded.total_points,
			updated_at = excluded.updated_at`,
		sink.NumCoupon, sink.Percent, sink.PointsToCoupon, sink.TotalPoints, ts,
	)
	if err != nil {
		return fmt.Errorf("upsert resource_sink_state: %w", err)
	}
	return nil
}

func insertResourceSinkSnapshot(ctx context.Context, db *sql.DB, sink frm.ResourceSink, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
		INSERT INTO resource_sink_snapshots (num_coupon, percent, total_points, captured_at)
		VALUES (?, ?, ?, ?)`,
		sink.NumCoupon, sink.Percent, sink.TotalPoints, ts,
	)
	if err != nil {
		return fmt.Errorf("insert resource_sink_snapshots: %w", err)
	}
	return nil
}

func upsertFactoryMachineState(ctx context.Context, db *sql.DB, machines []frm.FactoryMachine, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	for _, m := range machines {
		ingredientsJSON, err := json.Marshal(m.Ingredients)
		if err != nil {
			return err
		}
		productionJSON, err := json.Marshal(m.Production)
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, `
			INSERT INTO factory_machine_state (
				machine_id, building_type, recipe, manu_speed, is_configured, is_producing, is_paused,
				power_consumed, max_power_consumed, circuit_group_id, ingredients_json, production_json, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(machine_id) DO UPDATE SET
				building_type = excluded.building_type,
				recipe = excluded.recipe,
				manu_speed = excluded.manu_speed,
				is_configured = excluded.is_configured,
				is_producing = excluded.is_producing,
				is_paused = excluded.is_paused,
				power_consumed = excluded.power_consumed,
				max_power_consumed = excluded.max_power_consumed,
				circuit_group_id = excluded.circuit_group_id,
				ingredients_json = excluded.ingredients_json,
				production_json = excluded.production_json,
				updated_at = excluded.updated_at`,
			m.ID, buildingTypeFromMachine(m), m.Recipe, m.ManuSpeed,
			m.IsConfigured, m.IsProducing, m.IsPaused,
			m.PowerInfo.PowerConsumed, m.PowerInfo.MaxPowerConsumed, m.PowerInfo.CircuitGroupID,
			string(ingredientsJSON), string(productionJSON), ts,
		)
		if err != nil {
			return fmt.Errorf("upsert factory_machine_state %s: %w", m.ID, err)
		}
	}
	return nil
}

func upsertDroneState(ctx context.Context, db *sql.DB, drones []frm.Drone, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	for _, d := range drones {
		flyingSpeed := d.FlyingSpeed.Float64()
		maxSpeed := d.MaxSpeed.Float64()
		_, err := db.ExecContext(ctx, `
			INSERT INTO drone_state (
				drone_id, home_station, paired_station, has_paired_station, current_destination,
				flying_speed, max_speed, current_flying_mode, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(drone_id) DO UPDATE SET
				home_station = excluded.home_station,
				paired_station = excluded.paired_station,
				has_paired_station = excluded.has_paired_station,
				current_destination = excluded.current_destination,
				flying_speed = excluded.flying_speed,
				max_speed = excluded.max_speed,
				current_flying_mode = excluded.current_flying_mode,
				updated_at = excluded.updated_at`,
			d.ID, d.HomeStation, d.PairedStation, d.HasPairedStation, d.CurrentDestination,
			flyingSpeed, maxSpeed, d.CurrentFlyingMode, ts,
		)
		if err != nil {
			return fmt.Errorf("upsert drone_state %s: %w", d.ID, err)
		}
	}
	return nil
}

func upsertDoggoState(ctx context.Context, db *sql.DB, doggos []frm.Doggo, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	for _, d := range doggos {
		inventoryJSON, err := json.Marshal(d.Inventory)
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, `
			INSERT INTO doggo_state (doggo_id, name, inventory_json, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(doggo_id) DO UPDATE SET
				name = excluded.name,
				inventory_json = excluded.inventory_json,
				updated_at = excluded.updated_at`,
			d.ID, d.Name, string(inventoryJSON), ts,
		)
		if err != nil {
			return fmt.Errorf("upsert doggo_state %s: %w", d.ID, err)
		}
	}
	return nil
}

func appendCircuitSnapshots(ctx context.Context, db *sql.DB, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
		INSERT INTO circuit_snapshots (
			circuit_id, power_production, power_consumed, power_capacity, battery_percent, captured_at
		)
		SELECT circuit_id, power_production, power_consumed, power_capacity, battery_percent, ?
		FROM circuit_state`, ts)
	if err != nil {
		return fmt.Errorf("append circuit_snapshots: %w", err)
	}
	return nil
}

func pruneHistoryTables(ctx context.Context, db *sql.DB, retentionDays int, now time.Time) error {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := now.AddDate(0, 0, -retentionDays).UTC().Format(time.RFC3339)
	for _, table := range []string{"production_snapshots", "resource_sink_snapshots", "circuit_snapshots"} {
		query := fmt.Sprintf("DELETE FROM %s WHERE captured_at < ?", table)
		if _, err := db.ExecContext(ctx, query, cutoff); err != nil {
			return fmt.Errorf("prune %s: %w", table, err)
		}
	}
	return nil
}
