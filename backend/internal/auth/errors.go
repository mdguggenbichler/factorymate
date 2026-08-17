package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound         = errors.New("user not found")
	ErrSetupCompleted       = errors.New("setup already completed")
	ErrSessionNotFound      = errors.New("session not found")
	ErrForbidden            = errors.New("forbidden")
	ErrLastAdmin            = errors.New("cannot remove the last admin")
)
