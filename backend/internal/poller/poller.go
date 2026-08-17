package poller

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"factorymate/internal/frm"
)

// FastFetcher fetches fast-poll FRM data (implemented by frm.Client in production).
type FastFetcher interface {
	GetFast(ctx context.Context) frm.FastPollResult
}

// EventHandler receives detected events (wired to notify layer in M4).
type EventHandler func(ctx context.Context, event Event) error

// Poller runs the fast-poll loop at app_settings.poll_interval_seconds (§4.2).
type Poller struct {
	DB             *sql.DB
	Fetcher        FastFetcher
	ElevatorPhases *ElevatorPhases
	OnEvent        EventHandler
	Logger         *log.Logger
}

// New creates a Poller with FRM client config loaded from app_settings on each cycle.
func New(db *sql.DB, fetcher FastFetcher, phases *ElevatorPhases, onEvent EventHandler) *Poller {
	return &Poller{
		DB:             db,
		Fetcher:        fetcher,
		ElevatorPhases: phases,
		OnEvent:        onEvent,
		Logger:         log.Default(),
	}
}

// Run executes the poll loop until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	for {
		interval, err := p.pollInterval(ctx)
		if err != nil {
			p.logf("load poll interval: %v", err)
			interval = 20 * time.Second
		}

		if err := p.Poll(ctx); err != nil {
			p.logf("poll cycle: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (p *Poller) pollInterval(ctx context.Context) (time.Duration, error) {
	settings, err := loadAppSettings(ctx, p.DB)
	if err != nil {
		return 0, err
	}
	sec := settings.PollIntervalSeconds
	if sec <= 0 {
		sec = 20
	}
	return time.Duration(sec) * time.Second, nil
}

// Poll runs a single fast-poll cycle.
func (p *Poller) Poll(ctx context.Context) error {
	result := p.Fetcher.GetFast(ctx)
	engine := &Engine{DB: p.DB, ElevatorPhases: p.ElevatorPhases}
	events, err := engine.PollOnce(ctx, result, time.Now().UTC())
	if err != nil {
		return err
	}
	for _, ev := range events {
		if p.OnEvent != nil {
			if err := p.OnEvent(ctx, ev); err != nil {
				p.logf("event handler %s: %v", ev.MessageTypeKey, err)
			}
		}
	}
	return nil
}

func (p *Poller) logf(format string, args ...any) {
	if p.Logger != nil {
		p.Logger.Printf(format, args...)
	}
}

// FRMClientFromSettings builds an frm.Client from current app_settings.
func FRMClientFromSettings(ctx context.Context, db *sql.DB) (*frm.Client, error) {
	settings, err := loadAppSettings(ctx, db)
	if err != nil {
		return nil, err
	}
	token := ""
	if settings.FRMAuthToken.Valid {
		token = settings.FRMAuthToken.String
	}
	if settings.FRMHost == "" {
		return nil, fmt.Errorf("frm_host not configured")
	}
	return frm.NewClient(frm.Config{
		Host:  settings.FRMHost,
		Port:  settings.FRMPort,
		Token: token,
	}), nil
}
