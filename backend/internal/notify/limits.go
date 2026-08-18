package notify

import "time"

// Discord embed limits (spec §5.4 validation).
const (
	discordTitleMaxLen       = 256
	discordDescriptionMaxLen = 4096
	discordFooterMaxLen      = 2048
	discordContentMaxLen     = 2000
	discordMaxFields         = 25
	discordFieldNameMaxLen   = 256
	discordFieldValueMaxLen  = 1024
	dmRateLimit              = 200 * time.Millisecond
)

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
