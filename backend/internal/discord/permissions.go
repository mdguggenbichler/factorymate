package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"factorymate/internal/auth"
	"factorymate/internal/connection"
)

// Command groups used in role_mappings_json bot_commands arrays (§10.2).
const (
	CommandGroupAdmin      = "admin"
	CommandGroupRegister   = "register"
	CommandGroupPlayer     = "player"
	CommandGroupConnection = "connection"
	CommandGroupMods       = "mods"
)

// LinkState describes a Discord user's registration/link status for permission checks.
type LinkState int

const (
	LinkStateUnregistered LinkState = iota
	LinkStatePendingApproval
	LinkStateActiveLinked
	LinkStateActiveNotLinked
)

type roleMappingsConfig struct {
	GuildID             string              `json:"guild_id"`
	RoleMappings        []roleMappingEntry  `json:"role_mappings"`
	DefaultFMRole       string              `json:"default_fm_role"`
	DefaultBotCommands  []string            `json:"default_bot_commands"`
	AllowSelfRegister   bool                `json:"allow_self_register"`
	AdminDiscordRoleIDs []string            `json:"admin_discord_role_ids"`
}

type roleMappingEntry struct {
	DiscordRoleID string   `json:"discord_role_id"`
	FMRole        string   `json:"fm_role"`
	BotCommands   []string `json:"bot_commands"`
}

type memberPermissions struct {
	FMRole       auth.Role
	CommandGroups map[string]bool
	IsAdmin      bool
	AllowRegister bool
}

// LoadRoleMappings parses discord.role_mappings_json.
func LoadRoleMappings(ctx context.Context, db *sql.DB) (roleMappingsConfig, error) {
	raw, err := GetSetting(ctx, db, KeyRoleMappingsJSON)
	if err != nil {
		return roleMappingsConfig{}, err
	}
	cfg := roleMappingsConfig{
		DefaultFMRole:      string(auth.RoleViewer),
		AllowSelfRegister:  true,
		DefaultBotCommands: []string{},
	}
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return roleMappingsConfig{}, fmt.Errorf("parse role mappings: %w", err)
	}
	if cfg.DefaultFMRole == "" {
		cfg.DefaultFMRole = string(auth.RoleViewer)
	}
	return cfg, nil
}

// effectiveMemberRoleIDs includes the implicit @everyone role (guild ID) that Discord omits from member.Roles.
func effectiveMemberRoleIDs(guildID string, memberRoleIDs []string) []string {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return memberRoleIDs
	}
	for _, id := range memberRoleIDs {
		if id == guildID {
			return memberRoleIDs
		}
	}
	out := make([]string, len(memberRoleIDs), len(memberRoleIDs)+1)
	copy(out, memberRoleIDs)
	return append(out, guildID)
}

// ResolveMemberPermissions computes FM role and allowed command groups for a guild member.
func ResolveMemberPermissions(ctx context.Context, db *sql.DB, memberRoleIDs []string) (memberPermissions, error) {
	cfg, err := LoadRoleMappings(ctx, db)
	if err != nil {
		return memberPermissions{}, err
	}

	perms := memberPermissions{
		FMRole:        auth.Role(cfg.DefaultFMRole),
		CommandGroups: make(map[string]bool),
		AllowRegister: cfg.AllowSelfRegister,
	}
	for _, g := range cfg.DefaultBotCommands {
		perms.CommandGroups[g] = true
	}
	if cfg.AllowSelfRegister {
		perms.CommandGroups[CommandGroupRegister] = true
	}

	adminRoleSet := make(map[string]bool)
	for _, id := range cfg.AdminDiscordRoleIDs {
		adminRoleSet[id] = true
	}
	for _, envID := range adminRoleIDsFromEnv() {
		adminRoleSet[envID] = true
	}

	effectiveRoles := effectiveMemberRoleIDs(cfg.GuildID, memberRoleIDs)
	matched := false
	for _, memberRole := range effectiveRoles {
		if adminRoleSet[memberRole] {
			perms.IsAdmin = true
		}
		for _, mapping := range cfg.RoleMappings {
			if mapping.DiscordRoleID != memberRole {
				continue
			}
			matched = true
			if mapping.FMRole != "" {
				perms.FMRole = auth.Role(mapping.FMRole)
			}
			for _, g := range mapping.BotCommands {
				perms.CommandGroups[g] = true
			}
			if contains(mapping.BotCommands, CommandGroupAdmin) {
				perms.IsAdmin = true
			}
		}
	}

	if !matched && len(cfg.RoleMappings) > 0 {
		// Member has no mapped role — only default commands apply.
	}
	return perms, nil
}

// CanRunAdminCommand allows Discord admin role mapping or linked FM admin (§6.3, §10.2).
func CanRunAdminCommand(perms memberPermissions, state LinkState, fmUser *auth.User) bool {
	if fmUser != nil && fmUser.Role == auth.RoleAdmin {
		return true
	}
	return CanRunCommand(perms, CommandGroupAdmin, state)
}

// CanRunCommand applies §10.2 access rules.
func CanRunCommand(perms memberPermissions, group string, state LinkState) bool {
	if group == "" {
		return true
	}

	switch group {
	case CommandGroupRegister:
		switch state {
		case LinkStateUnregistered:
			return perms.CommandGroups[group] && perms.AllowRegister
		case LinkStateActiveLinked, LinkStateActiveNotLinked:
			return perms.IsAdmin
		default:
			return false
		}
	case CommandGroupAdmin:
		return perms.IsAdmin
	case CommandGroupPlayer, CommandGroupConnection, CommandGroupMods:
		// Active linked users may run player-group commands regardless of Discord role mapping (§10.2).
		return state == LinkStateActiveLinked
	default:
		if !perms.CommandGroups[group] && !(perms.IsAdmin && group == CommandGroupAdmin) {
			return false
		}
		return perms.CommandGroups[group]
	}
}

// FMRoleForMember returns the FM role to assign at registration.
func FMRoleForMember(ctx context.Context, db *sql.DB, memberRoleIDs []string) (auth.Role, error) {
	perms, err := ResolveMemberPermissions(ctx, db, memberRoleIDs)
	if err != nil {
		return auth.RoleViewer, err
	}
	return perms.FMRole, nil
}

func adminRoleIDsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("DISCORD_ADMIN_ROLE_IDS"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if id := strings.TrimSpace(p); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// LogBotCommand writes an audit row to bot_command_log.
func LogBotCommand(ctx context.Context, db *sql.DB, externalUserID, commandName string, success bool, detail string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO bot_command_log (external_platform, external_user_id, command_name, success, detail, created_at)
		VALUES ('discord', ?, ?, ?, ?, ?)`,
		externalUserID, commandName, success, connection.RedactForLog(detail), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert bot_command_log: %w", err)
	}
	return nil
}

// PublicURL returns the dashboard base URL for bot copy.
func PublicURL() string {
	if v := strings.TrimSpace(os.Getenv("FACTORYMATE_PUBLIC_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://factorymate.example.com"
}
