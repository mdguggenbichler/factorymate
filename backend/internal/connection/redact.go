package connection

import (
	"encoding/json"
	"regexp"
	"strings"

	"factorymate/internal/notify"
)

var (
	passwordLinePattern    = regexp.MustCompile(`(?i)(Password:\s*).+`)
	passwordPlainPattern   = regexp.MustCompile(`(?i)password[:\s]+[^\s\n]+`)
	gamePasswordAssign     = regexp.MustCompile(`(?i)game_password[=:]\s*[^\s,\n"]+`)
)

// RedactForLog removes game_password values from log-oriented strings (§8.6).
func RedactForLog(s string) string {
	s = notify.RedactForLog(s)
	s = gamePasswordAssign.ReplaceAllString(s, "game_password=[REDACTED]")
	s = passwordLinePattern.ReplaceAllString(s, "${1}[REDACTED]")
	s = passwordPlainPattern.ReplaceAllString(s, "password: [REDACTED]")
	return s
}

// RedactDetailsJSON returns JSON with password redacted for audit logs.
func RedactDetailsJSON(d Details) string {
	copy := d
	if copy.GamePassword != "" {
		copy.GamePassword = "[REDACTED]"
	}
	raw, err := json.Marshal(copy)
	if err != nil {
		return ""
	}
	return string(raw)
}

// ChangedFields lists field names touched by an update (no secret values).
func ChangedFields(old, new Details, input UpdateInput) []string {
	var fields []string
	if input.GameHost != nil && strings.TrimSpace(*input.GameHost) != old.GameHost {
		fields = append(fields, "game_host")
	}
	if input.GamePort != nil && *input.GamePort != old.GamePort {
		fields = append(fields, "game_port")
	}
	if input.ClearPassword || input.GamePassword != nil {
		fields = append(fields, "game_password")
	}
	if input.Notes != nil && strings.TrimSpace(*input.Notes) != old.Notes {
		fields = append(fields, "notes")
	}
	return fields
}
