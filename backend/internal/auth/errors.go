package auth

import "errors"

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUserNotFound          = errors.New("user not found")
	ErrSetupCompleted        = errors.New("setup already completed")
	ErrSessionNotFound       = errors.New("session not found")
	ErrForbidden             = errors.New("forbidden")
	ErrLastAdmin             = errors.New("cannot remove the last admin")
	ErrPendingApproval       = errors.New("account pending approval")
	ErrOAuthNotConfigured    = errors.New("discord oauth is not configured")
	ErrInvalidOAuthState     = errors.New("invalid or expired oauth state")
	ErrExternalAlreadyLinked = errors.New("external identity already linked")
	ErrDiscordNotLinked      = errors.New("discord account is not registered")
	ErrNoPassword            = errors.New("account has no password")
)
