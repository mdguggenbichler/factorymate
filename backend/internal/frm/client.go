package frm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const requestTimeout = 5 * time.Second

// Fast endpoints polled every poll_interval_seconds (§4.1).
var fastEndpoints = []string{
	"getPlayer",
	"getPower",
	"getSchematics",
	"getSpaceElevator",
	"getResearchTrees",
	"getTrains",
	"getVehicles",
}

// Slow endpoints polled on the production snapshot cadence (§4.1).
var slowEndpoints = []string{
	"getProdStats",
	"getResourceSink",
	"getFactory",
	"getDrone",
	"getDoggo",
}

// Client calls FRM read endpoints with a fixed timeout and no retries.
type Client struct {
	cfg  Config
	http *http.Client
}

// NewClient constructs an FRM client from app_settings-derived config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// FastPollResult holds fast-poll data with per-endpoint errors for unreachable detection (M3).
type FastPollResult struct {
	Players    []Player
	Power      []Circuit
	Schematics []Schematic
	Elevators  []Elevator
	Research   []ResearchTree
	Trains     []Train
	Vehicles   []Vehicle
	Errors     map[string]error
}

// Reachable reports whether every fast-poll endpoint succeeded (§4.2 server_online/offline).
func (r FastPollResult) Reachable() bool {
	return len(r.Errors) == 0
}

// SlowPollResult holds slow-poll data with per-endpoint errors.
type SlowPollResult struct {
	ProdStats    []ProdStat
	ResourceSink *ResourceSink
	Factory      []FactoryMachine
	Drones       []Drone
	Doggos       []Doggo
	Errors       map[string]error
}

// GetFast fetches all fast-poll endpoints in parallel. Partial failures are recorded in Errors.
func (c *Client) GetFast(ctx context.Context) FastPollResult {
	result := FastPollResult{Errors: make(map[string]error)}
	var mu sync.Mutex
	var wg sync.WaitGroup

	type job struct {
		name string
		run  func() error
	}

	jobs := []job{
		{"getPlayer", func() error {
			var v []Player
			if err := c.get(ctx, "getPlayer", &v); err != nil {
				return err
			}
			result.Players = v
			return nil
		}},
		{"getPower", func() error {
			var v []Circuit
			if err := c.get(ctx, "getPower", &v); err != nil {
				return err
			}
			result.Power = v
			return nil
		}},
		{"getSchematics", func() error {
			var v []Schematic
			if err := c.get(ctx, "getSchematics", &v); err != nil {
				return err
			}
			result.Schematics = v
			return nil
		}},
		{"getSpaceElevator", func() error {
			var v []Elevator
			if err := c.get(ctx, "getSpaceElevator", &v); err != nil {
				return err
			}
			result.Elevators = v
			return nil
		}},
		{"getResearchTrees", func() error {
			var v []ResearchTree
			if err := c.get(ctx, "getResearchTrees", &v); err != nil {
				return err
			}
			result.Research = v
			return nil
		}},
		{"getTrains", func() error {
			var v []Train
			if err := c.get(ctx, "getTrains", &v); err != nil {
				return err
			}
			result.Trains = v
			return nil
		}},
		{"getVehicles", func() error {
			var v []Vehicle
			if err := c.get(ctx, "getVehicles", &v); err != nil {
				return err
			}
			result.Vehicles = v
			return nil
		}},
	}

	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			if err := j.run(); err != nil {
				mu.Lock()
				result.Errors[j.name] = err
				mu.Unlock()
			}
		}(j)
	}
	wg.Wait()
	return result
}

// GetSlow fetches all slow-poll endpoints. getResourceSink uses the first array element (§4.1).
func (c *Client) GetSlow(ctx context.Context) SlowPollResult {
	result := SlowPollResult{Errors: make(map[string]error)}

	if err := c.get(ctx, "getProdStats", &result.ProdStats); err != nil {
		result.Errors["getProdStats"] = err
	}

	var sinks []ResourceSink
	if err := c.get(ctx, "getResourceSink", &sinks); err != nil {
		result.Errors["getResourceSink"] = err
	} else if len(sinks) > 0 {
		result.ResourceSink = &sinks[0]
	}

	if err := c.get(ctx, "getFactory", &result.Factory); err != nil {
		result.Errors["getFactory"] = err
	}

	if err := c.get(ctx, "getDrone", &result.Drones); err != nil {
		result.Errors["getDrone"] = err
	}

	if err := c.get(ctx, "getDoggo", &result.Doggos); err != nil {
		result.Errors["getDoggo"] = err
	}

	return result
}

func (c *Client) get(ctx context.Context, endpoint string, dest any) error {
	url := fmt.Sprintf("%s/%s", c.cfg.BaseURL(), endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if c.cfg.Token != "" {
		req.Header.Set("X-FRM-Authorization", c.cfg.Token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

// GetSessionInfo fetches FRM getSessionInfo (save/session metadata).
func (c *Client) GetSessionInfo(ctx context.Context) (SessionInfo, error) {
	var info SessionInfo
	if err := c.get(ctx, "getSessionInfo", &info); err != nil {
		return SessionInfo{}, err
	}
	return info, nil
}

// FastEndpoints returns the fast-poll endpoint names (for tests and M3).
func FastEndpoints() []string {
	out := make([]string, len(fastEndpoints))
	copy(out, fastEndpoints)
	return out
}

// SlowEndpoints returns the slow-poll endpoint names (for tests and M3).
func SlowEndpoints() []string {
	out := make([]string, len(slowEndpoints))
	copy(out, slowEndpoints)
	return out
}
