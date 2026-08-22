package poller

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	tcpProbeInterval   = 5 * time.Second
	tcpProbeTimeout    = 3 * time.Second
	connectionDetailsKey = "connection.details_json"
)

// RecoveryPhase is the poller gate state for FRM safe reconnect (§4.2).
type RecoveryPhase string

const (
	PhaseHealthy    RecoveryPhase = "healthy"
	PhaseDown       RecoveryPhase = "down"
	PhaseRecovering RecoveryPhase = "recovering"
)

// RecoveryAction tells the fast poller what to do after a recovering-cycle tick.
type RecoveryAction int

const (
	ActionWait RecoveryAction = iota
	ActionProbeSession
)

// DialFunc dials a TCP address (injectable for tests).
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Gate controls when background FRM HTTP polling is allowed (§4.2 safe reconnect).
type Gate struct {
	mu         sync.Mutex
	phase      RecoveryPhase
	graceUntil time.Time
	db         *sql.DB
	now        func() time.Time
	dial       DialFunc
	logf       func(format string, args ...any)
}

// NewGate constructs a gate, initializing phase from server_state.
func NewGate(db *sql.DB) (*Gate, error) {
	g := &Gate{
		db:  db,
		now: time.Now,
		dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, address)
		},
		logf: log.Printf,
	}
	ctx := context.Background()
	row, err := loadServerState(ctx, db)
	if err != nil {
		return nil, err
	}
	if row.Exists && row.RecoveryPhase != "" {
		g.phase = RecoveryPhase(row.RecoveryPhase)
	} else if row.Exists && row.ServerOnline.Valid && !row.ServerOnline.Bool {
		g.phase = PhaseDown
	} else {
		g.phase = PhaseHealthy
	}
	// Grace deadline is in-memory only; re-enter down to re-TCP-probe after restart.
	if g.phase == PhaseRecovering {
		g.phase = PhaseDown
		if err := upsertRecoveryPhase(ctx, db, PhaseDown, g.now()); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// Phase returns the current recovery phase.
func (g *Gate) Phase() RecoveryPhase {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.phase
}

// AllowFRMPoll reports whether background FRM HTTP calls are permitted.
func (g *Gate) AllowFRMPoll() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.phase == PhaseHealthy
}

// GraceElapsed reports whether the post-TCP grace window has passed (recovering only).
func (g *Gate) GraceElapsed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.phase != PhaseRecovering {
		return false
	}
	return !g.now().Before(g.graceUntil)
}

// OnFRMFailure transitions HEALTHY → DOWN and persists phase.
func (g *Gate) OnFRMFailure(ctx context.Context) error {
	g.mu.Lock()
	if g.phase == PhaseHealthy {
		g.phase = PhaseDown
	}
	g.mu.Unlock()
	return upsertRecoveryPhase(ctx, g.db, PhaseDown, g.now())
}

// OnFRMSuccess transitions to HEALTHY and persists phase.
func (g *Gate) OnFRMSuccess(ctx context.Context) error {
	g.mu.Lock()
	g.phase = PhaseHealthy
	g.graceUntil = time.Time{}
	g.mu.Unlock()
	return upsertRecoveryPhase(ctx, g.db, PhaseHealthy, g.now())
}

// OnRecoveryProbeFailure transitions RECOVERING → DOWN and persists phase.
func (g *Gate) OnRecoveryProbeFailure(ctx context.Context) error {
	g.mu.Lock()
	g.phase = PhaseDown
	g.graceUntil = time.Time{}
	g.mu.Unlock()
	return upsertRecoveryPhase(ctx, g.db, PhaseDown, g.now())
}

// RunDownCycle probes the game server TCP port from connection details.
// On success, transitions to RECOVERING and starts the grace timer.
func (g *Gate) RunDownCycle(ctx context.Context) time.Duration {
	host, port, ok := g.loadConnectionTarget(ctx)
	if !ok {
		g.logf("poller gate: connection details not configured; cannot TCP probe for recovery")
		return tcpProbeInterval
	}

	if !g.probeTCP(ctx, host, port) {
		return tcpProbeInterval
	}

	graceSec, err := g.loadGraceSeconds(ctx)
	if err != nil {
		g.logf("poller gate: load grace seconds: %v", err)
		graceSec = 60
	}

	g.mu.Lock()
	g.phase = PhaseRecovering
	g.graceUntil = g.now().Add(time.Duration(graceSec) * time.Second)
	g.mu.Unlock()

	if err := upsertRecoveryPhase(ctx, g.db, PhaseRecovering, g.now()); err != nil {
		g.logf("poller gate: persist recovering phase: %v", err)
	}
	return tcpProbeInterval
}

// RunRecoveringCycle returns the next action and sleep duration while recovering.
func (g *Gate) RunRecoveringCycle() (RecoveryAction, time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.phase != PhaseRecovering {
		return ActionWait, tcpProbeInterval
	}
	now := g.now()
	if now.Before(g.graceUntil) {
		return ActionWait, g.graceUntil.Sub(now)
	}
	return ActionProbeSession, 0
}

func (g *Gate) loadGraceSeconds(ctx context.Context) (int, error) {
	settings, err := loadAppSettings(ctx, g.db)
	if err != nil {
		return 0, err
	}
	sec := settings.FRMRecoveryGraceSeconds
	if sec <= 0 {
		sec = 60
	}
	return sec, nil
}

type connectionTargetJSON struct {
	GameHost string `json:"gameHost"`
	GamePort int    `json:"gamePort"`
}

func (g *Gate) loadConnectionTarget(ctx context.Context) (host string, port int, ok bool) {
	var raw string
	err := g.db.QueryRowContext(ctx, `
		SELECT value FROM app_setting_kv WHERE key = ?`, connectionDetailsKey,
	).Scan(&raw)
	if err == sql.ErrNoRows || strings.TrimSpace(raw) == "" || raw == "{}" {
		return "", 0, false
	}
	if err != nil {
		g.logf("poller gate: load connection details: %v", err)
		return "", 0, false
	}
	var d connectionTargetJSON
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		g.logf("poller gate: parse connection details: %v", err)
		return "", 0, false
	}
	host = strings.TrimSpace(d.GameHost)
	if host == "" || d.GamePort <= 0 {
		return "", 0, false
	}
	return host, d.GamePort, true
}

func (g *Gate) probeTCP(ctx context.Context, host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	probeCtx, cancel := context.WithTimeout(ctx, tcpProbeTimeout)
	defer cancel()
	conn, err := g.dial(probeCtx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// SetDialer replaces the TCP dialer (tests only).
func (g *Gate) SetDialer(d DialFunc) {
	g.dial = d
}

// SetNow replaces the clock (tests only).
func (g *Gate) SetNow(fn func() time.Time) {
	g.now = fn
}

// SetPhase sets phase directly (tests only).
func (g *Gate) SetPhase(p RecoveryPhase) {
	g.mu.Lock()
	g.phase = p
	g.mu.Unlock()
}

// SetGraceUntil sets grace deadline (tests only).
func (g *Gate) SetGraceUntil(t time.Time) {
	g.mu.Lock()
	g.graceUntil = t
	g.mu.Unlock()
}
