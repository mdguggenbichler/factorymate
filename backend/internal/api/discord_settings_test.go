package api_test

import (
	"context"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/db"
	"factorymate/internal/discord"
	"factorymate/internal/notify"
	"factorymate/internal/registration"
)

type mockSlashBot struct {
	cleared   []string
	registers int
}

func (m *mockSlashBot) Connected() bool { return true }

func (m *mockSlashBot) ClearSlashCommands(_ context.Context, guildID string) error {
	m.cleared = append(m.cleared, guildID)
	return nil
}

func (m *mockSlashBot) RegisterSlashCommands(context.Context) error {
	m.registers++
	return nil
}

func (m *mockSlashBot) ListGuildTextChannels(context.Context) ([]discord.Channel, error) {
	return nil, nil
}

func (m *mockSlashBot) InviteURL() (string, error) { return "", nil }

func (m *mockSlashBot) SendWelcomeDM(context.Context, string, string) {}

func (m *mockSlashBot) SendRegistrationDeclinedDM(context.Context, string, string) {}

func TestPutDiscordSettingsReregistersSlashCommandsOnGuildChange(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := discord.SetSetting(ctx, database, discord.KeyGuildID, "old-guild"); err != nil {
		t.Fatalf("seed guild: %v", err)
	}

	svc := auth.NewService(database)
	regSvc := registration.NewService(database, svc)
	handler := newTestHandler(database, svc, regSvc, notify.NewMockDiscordSession())
	mock := &mockSlashBot{}
	handler.SetDiscordBot(mock)
	router := newTestRouter(handler, svc)
	adminCookie := setupAdmin(t, router)

	guildID := "new-guild"
	resp := putJSONWithCookie(t, router, "/api/discord/settings", adminCookie, map[string]any{
		"guildId": guildID,
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("put discord settings status = %d", resp.StatusCode)
	}

	if len(mock.cleared) != 1 || mock.cleared[0] != "old-guild" {
		t.Fatalf("ClearSlashCommands calls = %v, want [old-guild]", mock.cleared)
	}
	if mock.registers != 1 {
		t.Fatalf("RegisterSlashCommands calls = %d, want 1", mock.registers)
	}

	got, err := discord.EffectiveGuildID(ctx, database)
	if err != nil {
		t.Fatalf("effective guild: %v", err)
	}
	if got != guildID {
		t.Fatalf("guild id = %q, want %q", got, guildID)
	}

	same := putJSONWithCookie(t, router, "/api/discord/settings", adminCookie, map[string]any{
		"guildId": guildID,
	})
	defer same.Body.Close()
	if same.StatusCode != 200 {
		t.Fatalf("put same guild status = %d", same.StatusCode)
	}
	if len(mock.cleared) != 1 || mock.registers != 1 {
		t.Fatalf("unchanged guild re-registered: cleared=%v registers=%d", mock.cleared, mock.registers)
	}
}
