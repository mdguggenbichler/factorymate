package poller

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"factorymate/internal/frm"
)

// Engine runs one fast-poll diff cycle (§4.2).
type Engine struct {
	DB             *sql.DB
	ElevatorPhases *ElevatorPhases
}

// PollOnce fetches fast-poll data, updates state tables, and returns detected events.
func (e *Engine) PollOnce(ctx context.Context, result frm.FastPollResult, now time.Time) ([]Event, error) {
	settings, err := loadAppSettings(ctx, e.DB)
	if err != nil {
		return nil, err
	}

	reachable := result.Reachable()
	var events []Event
	var session sessionSnapshot

	if reachable && strings.TrimSpace(settings.FRMHost) != "" {
		if snap, err := syncSessionFromFRM(ctx, e.DB, settings); err == nil {
			session = snap
			if snap.ServerName != "" {
				settings.ServerName = snap.ServerName
			}
		}
	}

	serverPrev, err := loadServerState(ctx, e.DB)
	if err != nil {
		return nil, err
	}

	if reachable {
		serverEvents := e.handleServerReachable(serverPrev, settings.ServerName, session.InGameTime)
		events = append(events, serverEvents...)
		if err := upsertServerState(ctx, e.DB, true, now); err != nil {
			return nil, err
		}

		playerEvents, err := e.processPlayers(ctx, result.Players, settings.ServerName, now)
		if err != nil {
			return nil, err
		}
		events = append(events, playerEvents...)

		circuitEvents, err := e.processCircuits(ctx, result.Power, settings.ServerName, now)
		if err != nil {
			return nil, err
		}
		events = append(events, circuitEvents...)

		schematicEvents, err := e.processSchematics(ctx, result.Schematics, now)
		if err != nil {
			return nil, err
		}
		events = append(events, schematicEvents...)

		elevatorEvents, err := e.processElevators(ctx, result.Elevators, now)
		if err != nil {
			return nil, err
		}
		events = append(events, elevatorEvents...)

		researchEvents, err := e.processResearch(ctx, result.Research, now)
		if err != nil {
			return nil, err
		}
		events = append(events, researchEvents...)

		trainEvents, err := e.processTrains(ctx, result.Trains, now)
		if err != nil {
			return nil, err
		}
		events = append(events, trainEvents...)

		vehicleEvents, err := e.processVehicles(ctx, result.Vehicles, settings.PollIntervalSeconds, now)
		if err != nil {
			return nil, err
		}
		events = append(events, vehicleEvents...)
	} else {
		// Unreachable: only server_offline transition; leave entity state untouched (§4.2).
		if serverPrev.Exists && serverPrev.ServerOnline.Valid && serverPrev.ServerOnline.Bool {
			vars := map[string]string{"ServerName": settings.ServerName}
			if session.InGameTime != "" {
				vars["InGameTime"] = session.InGameTime
			}
			events = append(events, Event{
				MessageTypeKey: "server_offline",
				Variables:      vars,
			})
		}
		if !serverPrev.Exists || !serverPrev.ServerOnline.Valid {
			// First observation while offline — baseline without event.
		}
		if err := upsertServerState(ctx, e.DB, false, now); err != nil {
			return nil, err
		}
	}

	if err := e.persistEventHistory(ctx, events, now); err != nil {
		return nil, err
	}

	return events, nil
}

func (e *Engine) handleServerReachable(prev serverStateRow, serverName, inGameTime string) []Event {
	if !prev.Exists || !prev.ServerOnline.Valid {
		return nil // First Observation — set true silently, no server_online event.
	}
	if !prev.ServerOnline.Bool {
		vars := map[string]string{"ServerName": serverName}
		if inGameTime != "" {
			vars["InGameTime"] = inGameTime
		}
		return []Event{{
			MessageTypeKey: "server_online",
			Variables:      vars,
		}}
	}
	return nil
}

func (e *Engine) processPlayers(ctx context.Context, players []frm.Player, serverName string, now time.Time) ([]Event, error) {
	onlineCount := countOnlinePlayers(players)
	ambiguous := ambiguousPlayerNames(players)
	var events []Event

	for _, p := range players {
		nameKey := normalizePlayerName(p.Name)
		_, isAmbiguous := ambiguous[nameKey]

		prev, err := loadPlayerStateDetailByID(ctx, e.DB, p.ID)
		if err != nil {
			return nil, err
		}

		if !prev.Exists && !isAmbiguous && nameKey != "" {
			merged, err := reconcilePlayerIdentity(ctx, e.DB, p)
			if err != nil {
				return nil, err
			}
			if merged.Exists {
				prev = merged
			}
		}

		var lastSeen *string
		if !prev.Exists {
			if err := upsertPlayerState(ctx, e.DB, p, nil, now); err != nil {
				return nil, err
			}
			if !isAmbiguous && nameKey != "" {
				if err := cleanupDuplicatePlayerRowsByName(ctx, e.DB, p.Name, p.ID); err != nil {
					return nil, err
				}
			}
			continue // First Observation
		}

		if !prev.Online && p.Online {
			events = append(events, Event{
				MessageTypeKey: "player_joined",
				Variables: map[string]string{
					"PlayerName":  p.Name,
					"OnlineCount": intToString(onlineCount),
					"ServerName":  serverName,
				},
			})
			if err := insertPlayerSessionEvent(ctx, e.DB, p.ID, p.Name, "joined", onlineCount, now); err != nil {
				return nil, err
			}
		} else if prev.Online && !p.Online {
			ts := now.UTC().Format(time.RFC3339)
			lastSeen = &ts
			events = append(events, Event{
				MessageTypeKey: "player_left",
				Variables: map[string]string{
					"PlayerName":  p.Name,
					"OnlineCount": intToString(onlineCount),
					"ServerName":  serverName,
				},
			})
			if err := insertPlayerSessionEvent(ctx, e.DB, p.ID, p.Name, "left", onlineCount, now); err != nil {
				return nil, err
			}
		}

		if err := upsertPlayerState(ctx, e.DB, p, lastSeen, now); err != nil {
			return nil, err
		}
		if !isAmbiguous && nameKey != "" {
			if err := cleanupDuplicatePlayerRowsByName(ctx, e.DB, p.Name, p.ID); err != nil {
				return nil, err
			}
		}
	}
	return events, nil
}

func (e *Engine) processCircuits(ctx context.Context, circuits []frm.Circuit, serverName string, now time.Time) ([]Event, error) {
	var events []Event
	for _, c := range circuits {
		prev, err := loadCircuitState(ctx, e.DB, c.CircuitGroupID)
		if err != nil {
			return nil, err
		}

		if !prev.Exists {
			if err := upsertCircuitState(ctx, e.DB, c, now); err != nil {
				return nil, err
			}
			continue
		}

		if !prev.Tripped && c.FuseTriggered {
			events = append(events, Event{
				MessageTypeKey: "fuse_tripped",
				Variables:      powerEventVars(c, serverName),
			})
			if err := insertPowerCircuitEvent(ctx, e.DB, c.CircuitGroupID, "fuse_tripped", now); err != nil {
				return nil, err
			}
		} else if prev.Tripped && !c.FuseTriggered {
			events = append(events, Event{
				MessageTypeKey: "power_restored",
				Variables:      powerEventVars(c, serverName),
			})
			if err := insertPowerCircuitEvent(ctx, e.DB, c.CircuitGroupID, "power_restored", now); err != nil {
				return nil, err
			}
		}

		if err := upsertCircuitState(ctx, e.DB, c, now); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (e *Engine) processSchematics(ctx context.Context, schematics []frm.Schematic, now time.Time) ([]Event, error) {
	var events []Event
	tiersWithUnlocks := make(map[int]struct{})

	for _, s := range schematics {
		prev, err := loadSchematicState(ctx, e.DB, s.ID)
		if err != nil {
			return nil, err
		}

		var purchasedAt *string
		if !prev.Exists {
			if s.Purchased {
				ts := now.UTC().Format(time.RFC3339)
				purchasedAt = &ts
			}
			if err := upsertSchematicState(ctx, e.DB, s, purchasedAt, now); err != nil {
				return nil, err
			}
			continue
		}

		if s.Type == "Milestone" && !prev.Purchased && s.Purchased {
			ts := now.UTC().Format(time.RFC3339)
			purchasedAt = &ts
			tiersWithUnlocks[s.TechTier] = struct{}{}
			events = append(events, Event{
				MessageTypeKey: "milestone_unlocked",
				Variables: map[string]string{
					"SchematicName": s.Name,
					"TechTier":      intToString(s.TechTier),
					"RecipeNames":   recipeNames(s.Recipes),
				},
			})
		}
		if s.Type == "Hard Drive" && prev.Locked && !s.Locked && !s.Purchased {
			events = append(events, Event{
				MessageTypeKey: "hard_drive_ready",
				Variables: map[string]string{
					"SchematicName": s.Name,
					"RecipeOptions": recipeOptions(s.Recipes),
				},
			})
		}

		if err := upsertSchematicState(ctx, e.DB, s, purchasedAt, now); err != nil {
			return nil, err
		}
	}

	for tier := range tiersWithUnlocks {
		if !hubTierComplete(schematics, tier) {
			continue
		}
		names := milestoneNamesForTier(schematics, tier)
		events = append(events, Event{
			MessageTypeKey: "hub_tier_complete",
			Variables: map[string]string{
				"TechTier":        intToString(tier),
				"MilestoneNames":  strings.Join(names, "\n"),
				"MilestoneCount":  intToString(len(names)),
			},
		})
	}

	return events, nil
}

func hubTierComplete(schematics []frm.Schematic, tier int) bool {
	found := false
	for _, s := range schematics {
		if s.Type != "Milestone" || s.TechTier != tier {
			continue
		}
		found = true
		if !s.Purchased {
			return false
		}
	}
	return found
}

func milestoneNamesForTier(schematics []frm.Schematic, tier int) []string {
	names := make([]string, 0)
	for _, s := range schematics {
		if s.Type == "Milestone" && s.TechTier == tier {
			names = append(names, s.Name)
		}
	}
	return names
}

func (e *Engine) processElevators(ctx context.Context, elevators []frm.Elevator, now time.Time) ([]Event, error) {
	var events []Event
	for _, el := range elevators {
		prev, err := loadElevatorState(ctx, e.DB, el.ID)
		if err != nil {
			return nil, err
		}

		var phaseNumber *int
		if phase, ok := e.ElevatorPhases.LookupPhase(el.CurrentPhase); ok {
			phaseNumber = &phase
		} else if len(el.CurrentPhase) > 0 {
			rawJSON, err := jsonMarshal(el.CurrentPhase)
			if err != nil {
				return nil, err
			}
			classNamesJSON, err := SortedClassNamesJSON(el.CurrentPhase)
			if err != nil {
				return nil, err
			}
			if err := insertElevatorPhaseUnknown(ctx, e.DB, rawJSON, classNamesJSON, now); err != nil {
				return nil, err
			}
		}

		if !prev.Exists {
			if err := upsertElevatorState(ctx, e.DB, el, phaseNumber, now); err != nil {
				return nil, err
			}
			continue
		}

		if !prev.UpgradeReady && el.UpgradeReady {
			vars := map[string]string{"ElevatorName": el.Name}
			if phaseNumber != nil {
				vars["PhaseNumber"] = intToString(*phaseNumber)
			}
			if len(el.CurrentPhase) > 0 {
				vars["PhaseRequirements"] = formatPhaseRequirements(el.CurrentPhase)
			}
			events = append(events, Event{
				MessageTypeKey: "elevator_phase_complete",
				Variables:      vars,
			})
		}

		if prev.UpgradeReady && !el.UpgradeReady {
			vars := map[string]string{"ElevatorName": el.Name}
			if prev.PhaseNumber != nil {
				vars["PhaseNumber"] = intToString(*prev.PhaseNumber)
			}
			if prev.CurrentPhaseJSON != "" {
				var items []frm.PhaseItem
				if err := json.Unmarshal([]byte(prev.CurrentPhaseJSON), &items); err == nil && len(items) > 0 {
					vars["PhaseRequirements"] = formatPhaseRequirements(items)
				}
			}
			events = append(events, Event{
				MessageTypeKey: "elevator_phase_done",
				Variables:      vars,
			})
		}

		if err := upsertElevatorState(ctx, e.DB, el, phaseNumber, now); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (e *Engine) processResearch(ctx context.Context, trees []frm.ResearchTree, now time.Time) ([]Event, error) {
	var events []Event
	for _, tree := range trees {
		if len(tree.Nodes) == 0 {
			continue
		}
		seenIDs := make([]string, 0, len(tree.Nodes))
		for _, node := range tree.Nodes {
			seenIDs = append(seenIDs, node.ID)
			prev, err := loadResearchNodeState(ctx, e.DB, tree.Name, node.ID)
			if err != nil {
				return nil, err
			}

			if !prev.Exists {
				if err := upsertResearchNodeState(ctx, e.DB, tree.Name, node, now); err != nil {
					return nil, err
				}
				continue
			}

			if node.State == "Purchased" && prev.State != "Purchased" {
				events = append(events, Event{
					MessageTypeKey: "research_unlocked",
					Variables: map[string]string{
						"NodeName":     node.Name,
						"TreeName":     tree.Name,
						"TechTier":     intToString(node.TechTier),
						"ResearchCost": formatResearchCost(node.Cost),
					},
				})
			}

			if err := upsertResearchNodeState(ctx, e.DB, tree.Name, node, now); err != nil {
				return nil, err
			}
		}
		if err := deleteResearchNodesNotInTree(ctx, e.DB, tree.Name, seenIDs); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (e *Engine) processTrains(ctx context.Context, trains []frm.Train, now time.Time) ([]Event, error) {
	var events []Event
	for _, t := range trains {
		prev, err := loadTrainState(ctx, e.DB, t.ID)
		if err != nil {
			return nil, err
		}

		if !prev.Exists {
			if err := upsertTrainState(ctx, e.DB, t, now); err != nil {
				return nil, err
			}
			continue
		}

		if !prev.Derailed && t.Derailed {
			events = append(events, Event{
				MessageTypeKey: "train_derailed",
				Variables: map[string]string{
					"TrainName":   t.Name,
					"StationName": t.TrainStation,
					"TrainStatus": t.Status,
					"SelfDriving": humanizeTrainError(t.SelfDriving),
				},
			})
		}

		if err := upsertTrainState(ctx, e.DB, t, now); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (e *Engine) processVehicles(ctx context.Context, vehicles []frm.Vehicle, pollIntervalSec int, now time.Time) ([]Event, error) {
	var events []Event
	for _, v := range vehicles {
		vehicleID := v.ID.String()
		prev, err := loadVehicleState(ctx, e.DB, vehicleID)
		if err != nil {
			return nil, err
		}

		fuelTotal := totalFuelAmount(v)
		fuelEmpty := fuelTotal == 0
		rawStuckCandidate := (v.IsAutoPilot() || v.FollowingPath) && v.ForwardSpeed < 0.5

		var lowSpeedSince *string
		stuck := false

		if !prev.Exists {
			if err := upsertVehicleState(ctx, e.DB, v, fuelEmpty, nil, false, now); err != nil {
				return nil, err
			}
			continue // First Observation — no debounce start
		}

		lowSpeedSince, stuck = computeVehicleStuck(prev, rawStuckCandidate, pollIntervalSec, now)

		if !prev.FuelEmpty && fuelEmpty {
			vtype := v.Type()
			events = append(events, Event{
				MessageTypeKey: "vehicle_out_of_fuel",
				Variables: map[string]string{
					"VehicleType":  vtype,
					"VehicleName":  v.DisplayName(),
					"Driver":       formatDriver(v.Driver),
					"ForwardSpeed": formatForwardSpeed(v.ForwardSpeed),
				},
			})
		}

		if !prev.Stuck && stuck {
			vtype := v.Type()
			events = append(events, Event{
				MessageTypeKey: "vehicle_stuck",
				Variables: map[string]string{
					"VehicleType":  vtype,
					"VehicleName":  v.DisplayName(),
					"Driver":       formatDriver(v.Driver),
					"ForwardSpeed": formatForwardSpeed(v.ForwardSpeed),
				},
			})
		}

		if err := upsertVehicleState(ctx, e.DB, v, fuelEmpty, lowSpeedSince, stuck, now); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func computeVehicleStuck(prev vehicleStateRow, rawCandidate bool, pollIntervalSec int, now time.Time) (*string, bool) {
	if !rawCandidate {
		return nil, false
	}

	if !prev.LowSpeedSince.Valid || prev.LowSpeedSince.String == "" {
		ts := now.UTC().Format(time.RFC3339)
		return &ts, false
	}

	firstObs, err := time.Parse(time.RFC3339, prev.LowSpeedSince.String)
	if err != nil {
		ts := now.UTC().Format(time.RFC3339)
		return &ts, false
	}

	// 3 consecutive polls: debounce elapses after 2 full intervals from first observation.
	elapsed := now.Sub(firstObs)
	threshold := time.Duration(pollIntervalSec*2) * time.Second
	if elapsed >= threshold {
		low := prev.LowSpeedSince.String
		return &low, true
	}

	low := prev.LowSpeedSince.String
	return &low, prev.Stuck
}

func (e *Engine) persistEventHistory(ctx context.Context, events []Event, now time.Time) error {
	// Player and power history are written inline during detection.
	_ = ctx
	_ = events
	_ = now
	return nil
}

func jsonMarshal(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
