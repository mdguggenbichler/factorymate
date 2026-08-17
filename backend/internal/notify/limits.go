package notify

// Discord embed limits (spec §5.4 validation).
const (
	discordTitleMaxLen       = 256
	discordDescriptionMaxLen = 4096
	discordMaxFields         = 25
	discordFieldNameMaxLen   = 256
	discordFieldValueMaxLen  = 1024
)

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
