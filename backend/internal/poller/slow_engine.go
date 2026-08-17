package poller

import (
	"context"
	"database/sql"
	"log"
	"time"

	"factorymate/internal/frm"
)

// SlowEngine runs one slow-poll cycle (§4.1): dashboard state + history snapshots.
type SlowEngine struct {
	DB     *sql.DB
	Logger *log.Logger
}

// PollOnce updates slow-poll tables from FRM data and circuit_state snapshots.
// Endpoint failures are logged and skip only their tables; server_state is never touched.
func (e *SlowEngine) PollOnce(ctx context.Context, result frm.SlowPollResult, now time.Time) error {
	settings, err := loadAppSettings(ctx, e.DB)
	if err != nil {
		return err
	}

	if err, ok := result.Errors["getProdStats"]; ok {
		e.logf("slow poll getProdStats: %v", err)
	} else {
		if err := upsertProdStatsState(ctx, e.DB, result.ProdStats, now); err != nil {
			e.logf("prod_stats_state: %v", err)
		} else if err := insertProductionSnapshots(ctx, e.DB, result.ProdStats, now); err != nil {
			e.logf("production_snapshots: %v", err)
		}
	}

	if err, ok := result.Errors["getResourceSink"]; ok {
		e.logf("slow poll getResourceSink: %v", err)
	} else if result.ResourceSink != nil {
		if err := upsertResourceSinkState(ctx, e.DB, *result.ResourceSink, now); err != nil {
			e.logf("resource_sink_state: %v", err)
		} else if err := insertResourceSinkSnapshot(ctx, e.DB, *result.ResourceSink, now); err != nil {
			e.logf("resource_sink_snapshots: %v", err)
		}
	}

	if err, ok := result.Errors["getFactory"]; ok {
		e.logf("slow poll getFactory: %v", err)
	} else if err := upsertFactoryMachineState(ctx, e.DB, result.Factory, now); err != nil {
		e.logf("factory_machine_state: %v", err)
	}

	if err, ok := result.Errors["getDrone"]; ok {
		e.logf("slow poll getDrone: %v", err)
	} else if err := upsertDroneState(ctx, e.DB, result.Drones, now); err != nil {
		e.logf("drone_state: %v", err)
	}

	if err, ok := result.Errors["getDoggo"]; ok {
		e.logf("slow poll getDoggo: %v", err)
	} else if err := upsertDoggoState(ctx, e.DB, result.Doggos, now); err != nil {
		e.logf("doggo_state: %v", err)
	}

	if err := appendCircuitSnapshots(ctx, e.DB, now); err != nil {
		e.logf("circuit_snapshots: %v", err)
	}

	if err := pruneHistoryTables(ctx, e.DB, settings.ProductionSnapshotRetentionDays, now); err != nil {
		e.logf("prune history: %v", err)
	}

	return nil
}

func (e *SlowEngine) logf(format string, args ...any) {
	if e.Logger != nil {
		e.Logger.Printf(format, args...)
	}
}
