package poller

import (
	"context"
	"database/sql"
	"log"
	"time"

	"factorymate/internal/frm"
)

// SlowFetcher fetches slow-poll FRM data (implemented by frm.Client in production).
type SlowFetcher interface {
	GetSlow(ctx context.Context) frm.SlowPollResult
}

// SlowPoller runs the slow-poll loop at app_settings.production_snapshot_interval_seconds (§4.1).
type SlowPoller struct {
	DB      *sql.DB
	Fetcher SlowFetcher
	Logger  *log.Logger
}

// NewSlowPoller creates a SlowPoller.
func NewSlowPoller(db *sql.DB, fetcher SlowFetcher) *SlowPoller {
	return &SlowPoller{
		DB:      db,
		Fetcher: fetcher,
		Logger:  log.Default(),
	}
}

// Run executes the slow-poll loop until ctx is cancelled.
func (p *SlowPoller) Run(ctx context.Context) {
	for {
		interval, err := p.snapshotInterval(ctx)
		if err != nil {
			p.logf("load snapshot interval: %v", err)
			interval = 300 * time.Second
		}

		if err := p.Poll(ctx); err != nil {
			p.logf("slow poll cycle: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (p *SlowPoller) snapshotInterval(ctx context.Context) (time.Duration, error) {
	settings, err := loadAppSettings(ctx, p.DB)
	if err != nil {
		return 0, err
	}
	sec := settings.ProductionSnapshotIntervalSeconds
	if sec <= 0 {
		sec = 300
	}
	return time.Duration(sec) * time.Second, nil
}

// Poll runs a single slow-poll cycle.
func (p *SlowPoller) Poll(ctx context.Context) error {
	result := p.Fetcher.GetSlow(ctx)
	engine := &SlowEngine{DB: p.DB, Logger: p.Logger}
	return engine.PollOnce(ctx, result, time.Now().UTC())
}

func (p *SlowPoller) logf(format string, args ...any) {
	if p.Logger != nil {
		p.Logger.Printf(format, args...)
	}
}
