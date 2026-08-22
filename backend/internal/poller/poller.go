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

// SessionProber checks FRM readiness with a lightweight getSessionInfo call.
type SessionProber interface {
	ProbeSession(ctx context.Context) error
}

// EventHandler receives detected events (wired to notify layer in M4).
type EventHandler func(ctx context.Context, event Event) error

// Poller runs the fast-poll loop at app_settings.poll_interval_seconds (§4.2).
type Poller struct {
	DB             *sql.DB
	Fetcher        FastFetcher
	SessionProber  SessionProber
	Gate           *Gate
	ElevatorPhases *ElevatorPhases
	OnEvent        EventHandler
	Logger         *log.Logger
}

// New creates a Poller with FRM client config loaded from app_settings on each cycle.
func New(db *sql.DB, fetcher FastFetcher, sessionProber SessionProber, gate *Gate, phases *ElevatorPhases, onEvent EventHandler) *Poller {
	return &Poller{
		DB:             db,
		Fetcher:        fetcher,
		SessionProber:  sessionProber,
		Gate:           gate,
		ElevatorPhases: phases,
		OnEvent:        onEvent,
		Logger:         log.Default(),
	}
}

// Run executes the poll loop until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		sleep := p.runCycle(ctx)
		if sleep <= 0 {
			sleep = tcpProbeInterval
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(sleep):
		}
	}
}

func (p *Poller) runCycle(ctx context.Context) time.Duration {
	if p.Gate == nil || p.Gate.AllowFRMPoll() {
		interval, err := p.pollInterval(ctx)
		if err != nil {
			p.logf("load poll interval: %v", err)
			interval = 20 * time.Second
		}
		if err := p.pollHealthy(ctx); err != nil {
			p.logf("poll cycle: %v", err)
		}
		return interval
	}

	switch p.Gate.Phase() {
	case PhaseDown:
		return p.Gate.RunDownCycle(ctx)
	case PhaseRecovering:
		return p.runRecovering(ctx)
	default:
		return tcpProbeInterval
	}
}

func (p *Poller) pollHealthy(ctx context.Context) error {
	result := p.Fetcher.GetFast(ctx)
	if !result.Reachable() && p.Gate != nil {
		if err := p.Gate.OnFRMFailure(ctx); err != nil {
			p.logf("gate on FRM failure: %v", err)
		}
	}
	return p.runEngine(ctx, result)
}

func (p *Poller) runRecovering(ctx context.Context) time.Duration {
	action, sleep := p.Gate.RunRecoveringCycle()
	if action == ActionWait {
		if sleep <= 0 {
			sleep = time.Second
		}
		return sleep
	}

	if p.SessionProber == nil {
		if err := p.Gate.OnRecoveryProbeFailure(ctx); err != nil {
			p.logf("gate on recovery probe failure: %v", err)
		}
		return tcpProbeInterval
	}

	if err := p.SessionProber.ProbeSession(ctx); err != nil {
		p.logf("recovery session probe: %v", err)
		if gateErr := p.Gate.OnRecoveryProbeFailure(ctx); gateErr != nil {
			p.logf("gate on recovery probe failure: %v", gateErr)
		}
		return tcpProbeInterval
	}

	result := p.Fetcher.GetFast(ctx)
	if !result.Reachable() {
		if err := p.Gate.OnRecoveryProbeFailure(ctx); err != nil {
			p.logf("gate on recovery full poll failure: %v", err)
		}
		return tcpProbeInterval
	}

	if err := p.runEngine(ctx, result); err != nil {
		p.logf("recovery poll engine: %v", err)
		if gateErr := p.Gate.OnRecoveryProbeFailure(ctx); gateErr != nil {
			p.logf("gate on recovery engine failure: %v", gateErr)
		}
		return tcpProbeInterval
	}

	if err := p.Gate.OnFRMSuccess(ctx); err != nil {
		p.logf("gate on FRM success: %v", err)
	}

	interval, err := p.pollInterval(ctx)
	if err != nil {
		return 20 * time.Second
	}
	return interval
}

func (p *Poller) runEngine(ctx context.Context, result frm.FastPollResult) error {
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

// Poll runs a single fast-poll cycle (used by tests; respects gate when set).
func (p *Poller) Poll(ctx context.Context) error {
	if p.Gate != nil && !p.Gate.AllowFRMPoll() {
		return nil
	}
	return p.pollHealthy(ctx)
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

// SettingsSessionProber probes FRM via getSessionInfo using current app_settings.
type SettingsSessionProber struct {
	DB *sql.DB
}

// ProbeSession implements SessionProber.
func (s *SettingsSessionProber) ProbeSession(ctx context.Context) error {
	client, err := FRMClientFromSettings(ctx, s.DB)
	if err != nil {
		return err
	}
	_, err = client.GetSessionInfo(ctx)
	return err
}
