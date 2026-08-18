package registration

import "errors"

const (
	PlatformDiscord = "discord"
	SourceDiscord   = "discord"
)

var (
	ErrAlreadyRegistered   = errors.New("discord account already linked")
	ErrUserNotFound        = errors.New("user not found")
	ErrUsernameTaken       = errors.New("username already taken")
	ErrNotPendingApproval  = errors.New("user is not pending approval")
	ErrPlayerAlreadyLinked = errors.New("player already linked to another user")
	ErrInvalidExternal     = errors.New("invalid external identity")
)
