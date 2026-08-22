package savegame

import "errors"

var (
	ErrNotConfigured = errors.New("savegame: dedicated server API not configured")
	ErrRateLimited   = errors.New("savegame: rate limit exceeded")
	ErrNoActiveSave  = errors.New("savegame: no active session or autosave found")
	ErrUpstream      = errors.New("savegame: upstream API error")
)

const (
	ChannelWeb     = "web"
	ChannelDiscord = "discord"

	rateLimitWindow = 5 // minutes
	DiscordMaxBytes = 25 * 1024 * 1024
)
