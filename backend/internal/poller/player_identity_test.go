package poller_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"factorymate/internal/db"
	"factorymate/internal/frm"
	"factorymate/internal/poller"
)

func TestPollCleansDuplicatePlayersWithLinkedUser(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openPollerTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	seedGuggiDuplicates(t, ctx, database)

	phases, err := poller.LoadElevatorPhases(poller.DefaultElevatorPhasesPath())
	if err != nil {
		t.Fatalf("load phases: %v", err)
	}
	engine := &poller.Engine{DB: database, ElevatorPhases: phases}
	now := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)

	result := frm.FastPollResult{
		Players: []frm.Player{{ID: "Char_Player_C_2147456886", Name: "guggi", Online: false}},
	}
	if _, err := engine.PollOnce(ctx, result, now); err != nil {
		t.Fatalf("poll: %v", err)
	}

	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_state WHERE LOWER(name) = 'guggi'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 guggi row after poll cleanup, got %d", count)
	}

	var linkedID sql.NullString
	if err := database.QueryRowContext(ctx, `
		SELECT player_id FROM users WHERE username = 'ghotso'`).Scan(&linkedID); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if !linkedID.Valid || linkedID.String != "Char_Player_C_2147456886" {
		t.Fatalf("user player_id = %v, want Char_Player_C_2147456886", linkedID)
	}
}

func TestDedupePlayerStateByName(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openPollerTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	seedGuggiDuplicates(t, ctx, database)

	if err := poller.DedupePlayerStateByName(ctx, database); err != nil {
		t.Fatalf("dedupe: %v", err)
	}

	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_state WHERE LOWER(name) = 'guggi'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 guggi row, got %d", count)
	}

	var playerID string
	var online bool
	if err := database.QueryRowContext(ctx, `
		SELECT player_id, online FROM player_state WHERE LOWER(name) = 'guggi'`).Scan(&playerID, &online); err != nil {
		t.Fatalf("query canonical: %v", err)
	}
	if playerID != "Char_Player_C_2147456886" {
		t.Fatalf("canonical player_id = %q, want Char_Player_C_2147456886", playerID)
	}
	if online {
		t.Fatal("canonical row should be offline")
	}

	var linkedID sql.NullString
	if err := database.QueryRowContext(ctx, `
		SELECT player_id FROM users WHERE username = 'ghotso'`).Scan(&linkedID); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if !linkedID.Valid || linkedID.String != playerID {
		t.Fatalf("user player_id = %v, want %s", linkedID, playerID)
	}
}

func TestPlayerIDMigrationByName(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openPollerTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO player_state (player_id, name, online, last_seen_at)
		VALUES ('old-id', 'Guggi', 1, NULL)`); err != nil {
		t.Fatalf("insert old player: %v", err)
	}

	phases, err := poller.LoadElevatorPhases(poller.DefaultElevatorPhasesPath())
	if err != nil {
		t.Fatalf("load phases: %v", err)
	}
	engine := &poller.Engine{DB: database, ElevatorPhases: phases}
	now := time.Date(2026, 8, 20, 15, 26, 0, 0, time.UTC)

	leave := frm.FastPollResult{
		Players: []frm.Player{{ID: "new-id", Name: "Guggi", Online: false}},
	}
	events, err := engine.PollOnce(ctx, leave, now)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(events) != 1 || events[0].MessageTypeKey != "player_left" {
		t.Fatalf("expected player_left after ID migration, got %+v", events)
	}

	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_state WHERE LOWER(name) = 'guggi'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 row, got %d", count)
	}

	var playerID string
	if err := database.QueryRowContext(ctx, `
		SELECT player_id FROM player_state WHERE LOWER(name) = 'guggi'`).Scan(&playerID); err != nil {
		t.Fatalf("query: %v", err)
	}
	if playerID != "new-id" {
		t.Fatalf("player_id = %q, want new-id", playerID)
	}
}

func TestPlayerRenameSameID(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openPollerTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	phases, err := poller.LoadElevatorPhases(poller.DefaultElevatorPhasesPath())
	if err != nil {
		t.Fatalf("load phases: %v", err)
	}
	engine := &poller.Engine{DB: database, ElevatorPhases: phases}
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	baseline := frm.FastPollResult{
		Players: []frm.Player{{ID: "p1", Name: "Alice", Online: false}},
	}
	if _, err := engine.PollOnce(ctx, baseline, now); err != nil {
		t.Fatalf("baseline poll: %v", err)
	}

	renamed := frm.FastPollResult{
		Players: []frm.Player{{ID: "p1", Name: "Alicia", Online: false}},
	}
	if _, err := engine.PollOnce(ctx, renamed, now.Add(20*time.Second)); err != nil {
		t.Fatalf("rename poll: %v", err)
	}

	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM player_state`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 player row, got %d", count)
	}

	var name string
	if err := database.QueryRowContext(ctx, `
		SELECT name FROM player_state WHERE player_id = 'p1'`).Scan(&name); err != nil {
		t.Fatalf("query: %v", err)
	}
	if name != "Alicia" {
		t.Fatalf("name = %q, want Alicia", name)
	}
}

func TestAmbiguousSameNameInPoll(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openPollerTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	phases, err := poller.LoadElevatorPhases(poller.DefaultElevatorPhasesPath())
	if err != nil {
		t.Fatalf("load phases: %v", err)
	}
	engine := &poller.Engine{DB: database, ElevatorPhases: phases}
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	result := frm.FastPollResult{
		Players: []frm.Player{
			{ID: "steve-1", Name: "Steve", Online: true},
			{ID: "steve-2", Name: "Steve", Online: false},
		},
	}
	if _, err := engine.PollOnce(ctx, result, now); err != nil {
		t.Fatalf("poll: %v", err)
	}

	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM player_state WHERE LOWER(name) = 'steve'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("want 2 Steve rows for ambiguous poll, got %d", count)
	}
}

func seedGuggiDuplicates(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	if _, err := database.ExecContext(ctx, `
		INSERT INTO player_state (player_id, name, online, last_seen_at) VALUES
			('Char_Player_C_2147456886', 'guggi', 0, '2026-08-20T15:26:22Z'),
			('Char_Player_C_2147459436', 'guggi', 1, NULL),
			('Char_Player_C_2147478067', 'guggi', 1, '2026-08-19T15:20:50Z')`); err != nil {
		t.Fatalf("insert players: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO player_session_events (player_id, player_name, event_type, online_count, occurred_at) VALUES
			('Char_Player_C_2147478067', 'guggi', 'joined', 1, '2026-08-19T15:25:11Z'),
			('Char_Player_C_2147456886', 'guggi', 'left', 0, '2026-08-20T15:26:22Z')`); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, created_at, player_id, status)
		VALUES ('ghotso', 'hash', 'admin', '2026-01-01T00:00:00Z', 'Char_Player_C_2147478067', 'active')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}
