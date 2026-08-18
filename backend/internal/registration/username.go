package registration

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

var usernameSanitizeRE = regexp.MustCompile(`[^a-z0-9_-]+`)

const maxUsernameLen = 32

// DeriveUsername builds a base FM username from Discord display name and username.
// Sanitizes to [a-z0-9_-], max 32 characters. Prefer displayName when non-empty.
func DeriveUsername(displayName, discordUsername string) string {
	raw := strings.TrimSpace(displayName)
	if raw == "" {
		raw = strings.TrimSpace(discordUsername)
	}
	sanitized := sanitizeUsernameBase(raw)
	if sanitized == "" || sanitized == "user" {
		fallback := sanitizeUsernameBase(strings.TrimSpace(discordUsername))
		if fallback != "" && fallback != "user" {
			sanitized = fallback
		}
	}
	if sanitized == "" {
		sanitized = "user"
	}
	if len(sanitized) > maxUsernameLen {
		sanitized = strings.TrimRight(sanitized[:maxUsernameLen], "-")
		if sanitized == "" {
			sanitized = "user"
		}
	}
	return sanitized
}

func sanitizeUsernameBase(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = usernameSanitizeRE.ReplaceAllString(raw, "-")
	raw = strings.Trim(raw, "-")
	for strings.Contains(raw, "--") {
		raw = strings.ReplaceAll(raw, "--", "-")
	}
	return raw
}

// AllocateUsername returns an available username, auto-suffixing michael-2, michael-3, etc.
func AllocateUsername(ctx context.Context, db *sql.DB, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("username base is required")
	}
	if len(base) > maxUsernameLen {
		base = strings.TrimRight(base[:maxUsernameLen], "-")
		if base == "" {
			base = "user"
		}
	}

	for i := 0; i < 1000; i++ {
		candidate := base
		if i > 0 {
			suffix := fmt.Sprintf("-%d", i+1)
			prefixLen := maxUsernameLen - len(suffix)
			if prefixLen < 1 {
				return "", fmt.Errorf("username base too long for suffix")
			}
			prefix := base
			if len(prefix) > prefixLen {
				prefix = strings.TrimRight(prefix[:prefixLen], "-")
			}
			if prefix == "" {
				prefix = "user"
			}
			candidate = prefix + suffix
		}

		var taken int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username = ?`, candidate).Scan(&taken); err != nil {
			return "", fmt.Errorf("check username: %w", err)
		}
		if taken == 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate username for %q", base)
}
