package poller

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"factorymate/internal/frm"
)

func normalizePlayerName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// ambiguousPlayerNames returns lower-cased names that appear more than once in a poll.
func ambiguousPlayerNames(players []frm.Player) map[string]struct{} {
	counts := make(map[string]int)
	for _, p := range players {
		key := normalizePlayerName(p.Name)
		if key == "" {
			continue
		}
		counts[key]++
	}
	ambiguous := make(map[string]struct{})
	for name, n := range counts {
		if n > 1 {
			ambiguous[name] = struct{}{}
		}
	}
	return ambiguous
}

type playerStateDetail struct {
	PlayerID   string
	Name       string
	Online     bool
	LastSeenAt sql.NullString
	Exists     bool
}

func loadPlayerStateDetailByID(ctx context.Context, db *sql.DB, playerID string) (playerStateDetail, error) {
	var row playerStateDetail
	err := db.QueryRowContext(ctx, `
		SELECT player_id, name, online, last_seen_at
		FROM player_state WHERE player_id = ?`, playerID,
	).Scan(&row.PlayerID, &row.Name, &row.Online, &row.LastSeenAt)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	row.Exists = true
	return row, nil
}

// loadMergedPlayerStateByName aggregates duplicate same-name rows for transition detection.
func loadMergedPlayerStateByName(ctx context.Context, db *sql.DB, name string) (playerStateDetail, []string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT player_id, name, online, last_seen_at
		FROM player_state WHERE LOWER(name) = LOWER(?)
		ORDER BY player_id`, name)
	if err != nil {
		return playerStateDetail{}, nil, err
	}
	defer rows.Close()

	var merged playerStateDetail
	var ids []string
	for rows.Next() {
		var row playerStateDetail
		if err := rows.Scan(&row.PlayerID, &row.Name, &row.Online, &row.LastSeenAt); err != nil {
			return playerStateDetail{}, nil, err
		}
		ids = append(ids, row.PlayerID)
		if !merged.Exists {
			merged = row
			merged.Exists = true
			continue
		}
		if row.Online {
			merged.Online = true
		}
		if row.LastSeenAt.Valid {
			if !merged.LastSeenAt.Valid || row.LastSeenAt.String > merged.LastSeenAt.String {
				merged.LastSeenAt = row.LastSeenAt
			}
		}
	}
	if err := rows.Err(); err != nil {
		return playerStateDetail{}, nil, err
	}
	return merged, ids, nil
}

func relinkUsersToPlayerID(ctx context.Context, q execQuerier, newID string, oldIDs []string) error {
	for _, oldID := range oldIDs {
		if oldID == newID {
			continue
		}
		if _, err := q.ExecContext(ctx, `
			UPDATE users SET player_id = ? WHERE player_id = ?`, newID, oldID); err != nil {
			return fmt.Errorf("relink user player_id %s -> %s: %w", oldID, newID, err)
		}
	}
	return nil
}

func deletePlayerRowsByIDs(ctx context.Context, q execQuerier, ids []string, exceptID string) error {
	for _, id := range ids {
		if id == exceptID {
			continue
		}
		if _, err := q.ExecContext(ctx, `DELETE FROM player_state WHERE player_id = ?`, id); err != nil {
			return fmt.Errorf("delete player_state %s: %w", id, err)
		}
	}
	return nil
}

func deletePlayerRowsByNameExcept(ctx context.Context, q execQuerier, name, keepID string) error {
	_, err := q.ExecContext(ctx, `
		DELETE FROM player_state
		WHERE LOWER(name) = LOWER(?) AND player_id != ?`, name, keepID)
	if err != nil {
		return fmt.Errorf("delete duplicate player rows for %q: %w", name, err)
	}
	return nil
}

type execQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// reconcilePlayerIdentity relinks users and removes stale rows when FRM returns a new ID for an existing name.
func reconcilePlayerIdentity(ctx context.Context, db *sql.DB, p frm.Player) (playerStateDetail, error) {
	merged, oldIDs, err := loadMergedPlayerStateByName(ctx, db, p.Name)
	if err != nil {
		return playerStateDetail{}, err
	}
	if !merged.Exists {
		return playerStateDetail{}, nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return playerStateDetail{}, err
	}
	defer tx.Rollback()

	if err := relinkUsersToPlayerID(ctx, tx, p.ID, oldIDs); err != nil {
		return playerStateDetail{}, err
	}
	if err := deletePlayerRowsByIDs(ctx, tx, oldIDs, p.ID); err != nil {
		return playerStateDetail{}, err
	}
	if err := tx.Commit(); err != nil {
		return playerStateDetail{}, err
	}
	return merged, nil
}

type playerRowCandidate struct {
	PlayerID        string
	Name            string
	Online          bool
	LastSeenAt      sql.NullString
	LatestEventAt   sql.NullString
	UserLinked      bool
}

// DedupePlayerStateByName merges legacy duplicate player_state rows that share a name.
// Idempotent; safe to run on every startup.
func DedupePlayerStateByName(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
		SELECT LOWER(name) AS lname
		FROM player_state
		GROUP BY LOWER(name)
		HAVING COUNT(*) > 1`)
	if err != nil {
		return fmt.Errorf("query duplicate player names: %w", err)
	}
	defer rows.Close()

	var duplicateNames []string
	for rows.Next() {
		var lname string
		if err := rows.Scan(&lname); err != nil {
			return err
		}
		duplicateNames = append(duplicateNames, lname)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, lname := range duplicateNames {
		if err := dedupePlayerNameGroup(ctx, db, lname); err != nil {
			return err
		}
	}
	return nil
}

func dedupePlayerNameGroup(ctx context.Context, db *sql.DB, lname string) error {
	candidates, err := loadPlayerCandidates(ctx, db, lname)
	if err != nil {
		return err
	}
	if len(candidates) < 2 {
		return nil
	}

	canonical := pickCanonicalPlayer(candidates)
	var duplicateIDs []string
	for _, c := range candidates {
		if c.PlayerID != canonical.PlayerID {
			duplicateIDs = append(duplicateIDs, c.PlayerID)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := relinkUsersToPlayerID(ctx, tx, canonical.PlayerID, duplicateIDs); err != nil {
		return err
	}

	mergedLastSeen := canonical.LastSeenAt
	for _, c := range candidates {
		if c.LastSeenAt.Valid {
			if !mergedLastSeen.Valid || c.LastSeenAt.String > mergedLastSeen.String {
				mergedLastSeen = c.LastSeenAt
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE player_state SET name = ?, online = ?, last_seen_at = ?
		WHERE player_id = ?`,
		canonical.Name, canonical.Online, nullStringValue(mergedLastSeen), canonical.PlayerID,
	); err != nil {
		return fmt.Errorf("update canonical player_state: %w", err)
	}

	if err := deletePlayerRowsByIDs(ctx, tx, duplicateIDs, canonical.PlayerID); err != nil {
		return err
	}
	return tx.Commit()
}

func nullStringValue(v sql.NullString) any {
	if v.Valid {
		return v.String
	}
	return nil
}

func loadPlayerCandidates(ctx context.Context, db *sql.DB, lname string) ([]playerRowCandidate, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.player_id, p.name, p.online, p.last_seen_at,
			(SELECT MAX(e.occurred_at) FROM player_session_events e WHERE e.player_id = p.player_id),
			EXISTS(SELECT 1 FROM users u WHERE u.player_id = p.player_id)
		FROM player_state p
		WHERE LOWER(p.name) = ?
		ORDER BY p.player_id`, lname)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []playerRowCandidate
	for rows.Next() {
		var c playerRowCandidate
		if err := rows.Scan(&c.PlayerID, &c.Name, &c.Online, &c.LastSeenAt, &c.LatestEventAt, &c.UserLinked); err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

func pickCanonicalPlayer(candidates []playerRowCandidate) playerRowCandidate {
	best := candidates[0]
	for _, c := range candidates[1:] {
		if playerCandidateRank(c) > playerCandidateRank(best) {
			best = c
			continue
		}
		if playerCandidateRank(c) == playerCandidateRank(best) && c.PlayerID < best.PlayerID {
			best = c
		}
	}
	return best
}

func playerCandidateRank(c playerRowCandidate) int {
	rank := 0
	if c.LastSeenAt.Valid {
		rank += parseTimeRank(c.LastSeenAt.String)
	}
	if c.LatestEventAt.Valid {
		rank += parseTimeRank(c.LatestEventAt.String)
	}
	if !c.Online {
		rank += 1 // prefer offline over stale online
	}
	if c.UserLinked {
		rank += 1
	}
	return rank
}

func parseTimeRank(ts string) int {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return 0
	}
	return int(t.Unix())
}
