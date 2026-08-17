package discord_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"factorymate/internal/auth"
	"factorymate/internal/db"
	"factorymate/internal/discord"
	"factorymate/internal/registration"

	"github.com/bwmarrin/discordgo"
)

func TestApprovalButtonCustomIDs(t *testing.T) {
	userID, err := discord.ParseUserIDFromCustomID("btn_reg_approve:42", "btn_reg_approve:")
	if err != nil || userID != 42 {
		t.Fatalf("approve id = %d, err = %v", userID, err)
	}
	userID, err = discord.ParseUserIDFromCustomID("btn_reg_reject:7", "btn_reg_reject:")
	if err != nil || userID != 7 {
		t.Fatalf("reject id = %d, err = %v", userID, err)
	}
}

func TestCanRunCommandRoleMapping(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	mappings := map[string]any{
		"role_mappings": []map[string]any{
			{
				"discord_role_id": "role-viewer",
				"fm_role":         "viewer",
				"bot_commands":    []string{"register", "player"},
			},
			{
				"discord_role_id": "role-admin",
				"fm_role":         "admin",
				"bot_commands":    []string{"admin", "register", "player"},
			},
		},
		"allow_self_register": true,
		"default_fm_role":     "viewer",
	}
	raw, _ := json.Marshal(mappings)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES ('discord.role_mappings_json', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(raw)); err != nil {
		t.Fatalf("seed mappings: %v", err)
	}

	perms, err := discord.ResolveMemberPermissions(ctx, database, []string{"role-viewer"})
	if err != nil {
		t.Fatalf("resolve perms: %v", err)
	}
	if !discord.CanRunCommand(perms, discord.CommandGroupRegister, discord.LinkStateUnregistered) {
		t.Fatal("viewer should self-register")
	}
	if discord.CanRunCommand(perms, discord.CommandGroupAdmin, discord.LinkStateUnregistered) {
		t.Fatal("viewer should not run admin commands")
	}

	adminUser := &auth.User{Role: auth.RoleAdmin}
	if !discord.CanRunAdminCommand(perms, discord.LinkStateActiveLinked, adminUser) {
		t.Fatal("FM admin should bypass Discord admin role requirement")
	}
}

func TestCanRunCommand_PendingApprovalBlocked(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}

	mappings := map[string]any{
		"role_mappings": []map[string]any{
			{
				"discord_role_id": "role-viewer",
				"fm_role":         "viewer",
				"bot_commands":    []string{"register", "player", "connection", "mods"},
			},
		},
		"allow_self_register": true,
	}
	raw, _ := json.Marshal(mappings)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES ('discord.role_mappings_json', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(raw)); err != nil {
		t.Fatalf("seed mappings: %v", err)
	}

	perms, err := discord.ResolveMemberPermissions(ctx, database, []string{"role-viewer"})
	if err != nil {
		t.Fatalf("resolve perms: %v", err)
	}

	for _, group := range []string{
		discord.CommandGroupPlayer,
		discord.CommandGroupConnection,
		discord.CommandGroupMods,
	} {
		if discord.CanRunCommand(perms, group, discord.LinkStatePendingApproval) {
			t.Fatalf("pending approval user should not run %q commands", group)
		}
	}
}

func TestParseNotificationsOptionsDefaultsToView(t *testing.T) {
	action, category, enabled := discord.ParseNotificationsOptionsForTest(discordgo.ApplicationCommandInteractionData{})
	if action != "view" || category != "" || enabled != "" {
		t.Fatalf("defaults = (%q, %q, %q), want view, empty, empty", action, category, enabled)
	}
}

func TestApproveRegistrationViaService(t *testing.T) {
	t.Chdir("../..")
	ctx := context.Background()
	database := openTestDB(t)
	defer database.Close()
	if err := db.Init(ctx, database, db.SeedConfig{}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO app_setting_kv (key, value) VALUES ('registration.auto_approve', 'false')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		t.Fatalf("set auto approve: %v", err)
	}

	authSvc := auth.NewService(database)
	regSvc := registration.NewService(database, authSvc)
	admin, err := authSvc.CreateUser(ctx, "admin", "password123", auth.RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	_, err = database.ExecContext(ctx, `
		UPDATE users SET external_platform = 'discord', external_user_id = 'admin-discord'
		WHERE id = ?`, admin.ID)
	if err != nil {
		t.Fatalf("link admin discord: %v", err)
	}

	result, err := regSvc.Register(ctx, registration.RegisterParams{
		Username:          "newbie",
		Password:          "password123",
		PendingPlayerName: "Newbie",
		External: registration.ExternalIdentity{
			Platform: registration.PlatformDiscord,
			UserID:   "user-discord",
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	approved, err := regSvc.ApproveRegistration(ctx, result.User.ID, admin.ID)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != auth.StatusActive {
		t.Fatalf("status = %q", approved.Status)
	}

	var action string
	if err := database.QueryRowContext(ctx, `
		SELECT action FROM registration_audit_log WHERE user_id = ? ORDER BY id DESC LIMIT 1`,
		result.User.ID).Scan(&action); err != nil {
		t.Fatalf("audit log: %v", err)
	}
	if action != "approved" {
		t.Fatalf("audit action = %q", action)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database
}
