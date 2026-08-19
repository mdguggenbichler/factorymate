package notify_test

import (
	"context"
	"testing"
	"time"

	"factorymate/internal/notify"
)

func TestDispatcher_DMFanOutRespectsPrefs(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openDispatchTestDB(t)
	defer database.Close()

	mock := notify.NewMockDiscordSession()
	targetID := insertDiscordTarget(t, ctx, database, "channel-1")
	if err := assignTarget(t, ctx, database, "fuse_tripped", targetID); err != nil {
		t.Fatalf("assign target: %v", err)
	}

	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, created_at, status, external_platform, external_user_id)
		VALUES
			(10, 'power-on', 'hash', 'viewer', ?, 'active', 'discord', 'discord-on'),
			(11, 'power-off', 'hash', 'viewer', ?, 'active', 'discord', 'discord-off')`,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO user_notification_prefs (user_id, message_type_key, dm_enabled, updated_at)
		VALUES (10, 'fuse_tripped', 1, ?), (10, 'power_restored', 0, ?),
			(11, 'fuse_tripped', 0, ?), (11, 'power_restored', 0, ?)`,
		now.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339),
	); err != nil {
		t.Fatalf("insert prefs: %v", err)
	}

	d := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(mock),
	})
	d.Now = func() time.Time { return now }

	err := d.HandleEvent(ctx, "fuse_tripped", map[string]string{
		"CircuitID":       "1",
		"PowerProduction": "120",
		"PowerConsumed":   "95",
		"PowerCapacity":   "100",
		"BatteryPercent":  "68",
		"BatteryTimeEmpty": "2h",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	if len(mock.ChannelCalls) != 2 {
		t.Fatalf("channel calls = %d, want 2 (channel target + dm)", len(mock.ChannelCalls))
	}
	if len(mock.DMUserIDs) != 1 || mock.DMUserIDs[0] != "discord-on" {
		t.Fatalf("dm recipients = %v, want [discord-on]", mock.DMUserIDs)
	}

	var dmCount int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_log
		WHERE message_type_key = 'fuse_tripped' AND delivery_mode = 'dm'`,
	).Scan(&dmCount); err != nil {
		t.Fatalf("count dm logs: %v", err)
	}
	if dmCount != 1 {
		t.Fatalf("dm log rows = %d, want 1", dmCount)
	}
}

func TestDispatcher_PersonalPlayerDM(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openDispatchTestDB(t)
	defer database.Close()

	mock := notify.NewMockDiscordSession()
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC)

	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, created_at, status, external_platform, external_user_id, pending_player_name, dm_player_personal)
		VALUES (20, 'michael', 'hash', 'viewer', ?, 'active', 'discord', 'discord-michael', 'Michael', 1)`,
		now.Format(time.RFC3339),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	d := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(mock),
	})
	d.Now = func() time.Time { return now }

	err := d.HandleEvent(ctx, "player_joined", map[string]string{
		"PlayerName":  "Michael",
		"OnlineCount": "2",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	if len(mock.DMUserIDs) != 1 || mock.DMUserIDs[0] != "discord-michael" {
		t.Fatalf("dm recipients = %v, want [discord-michael]", mock.DMUserIDs)
	}
	if len(mock.ChannelCalls) != 1 {
		t.Fatalf("dm channel calls = %d, want 1", len(mock.ChannelCalls))
	}
	gotTitle := mock.ChannelCalls[0].Message.Embeds[0].Title
	if gotTitle != "👤 Your character joined the server" {
		t.Fatalf("personal dm title = %q", gotTitle)
	}
}

func TestDispatcher_NotifyPlayerAutoLinked(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openDispatchTestDB(t)
	defer database.Close()

	mock := notify.NewMockDiscordSession()
	d := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(mock),
	})

	err := d.NotifyPlayerAutoLinked(ctx, []notify.PlayerAutoLink{{
		ExternalUserID: "discord-linked",
		PlayerName:     "Guggi",
	}})
	if err != nil {
		t.Fatalf("NotifyPlayerAutoLinked: %v", err)
	}
	if len(mock.DMUserIDs) != 1 || mock.DMUserIDs[0] != "discord-linked" {
		t.Fatalf("dm recipients = %v", mock.DMUserIDs)
	}
}

func TestDispatcher_DMRegressionSkipsWhenTypeDisabled(t *testing.T) {
	t.Chdir("../..")

	ctx := context.Background()
	database := openDispatchTestDB(t)
	defer database.Close()

	mock := notify.NewMockDiscordSession()
	targetID := insertDiscordTarget(t, ctx, database, "channel-1")
	if err := assignTarget(t, ctx, database, "player_joined", targetID); err != nil {
		t.Fatalf("assign target: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE message_types SET enabled = 0 WHERE key = 'player_joined'`); err != nil {
		t.Fatalf("disable type: %v", err)
	}
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, created_at, status, external_platform, external_user_id)
		VALUES (30, 'viewer', 'hash', 'viewer', ?, 'active', 'discord', 'discord-viewer')`,
		now.Format(time.RFC3339),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO user_notification_prefs (user_id, message_type_key, dm_enabled, updated_at)
		VALUES (30, 'player_joined', 1, ?)`, now.Format(time.RFC3339)); err != nil {
		t.Fatalf("insert prefs: %v", err)
	}

	d := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(mock),
	})
	if err := d.HandleEvent(ctx, "player_joined", map[string]string{
		"PlayerName":  "Guggi",
		"OnlineCount": "1",
	}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(mock.ChannelCalls) != 0 || len(mock.DMUserIDs) != 0 {
		t.Fatal("disabled message type should skip channel and dm delivery")
	}
}
