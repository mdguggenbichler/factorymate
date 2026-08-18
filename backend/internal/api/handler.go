package api

import (
	"database/sql"

	"factorymate/internal/auth"
	"factorymate/internal/connection"
	"factorymate/internal/discord"
	"factorymate/internal/mods"
	"factorymate/internal/notifications"
	"factorymate/internal/notify"
	"factorymate/internal/registration"
)

type Handler struct {
	db            *sql.DB
	auth          *auth.Service
	registration  *registration.Service
	connection    *connection.Service
	mods          *mods.Service
	dispatcher    *notify.Dispatcher
	discord       *discord.Bot
	notifications *notifications.Service
}

func NewHandler(db *sql.DB, authSvc *auth.Service, bot *discord.Bot, regSvc *registration.Service, connSvc *connection.Service, modsSvc *mods.Service) *Handler {
	return newHandlerWithProvider(db, authSvc, bot, regSvc, connSvc, modsSvc, notify.NewDiscordProviderWithSessionFn(func() notify.DiscordSession {
		if bot == nil {
			return nil
		}
		return bot.Session()
	}))
}

// NewHandlerWithDiscordSession is for tests that need a mock Discord session without a live bot.
func NewHandlerWithDiscordSession(db *sql.DB, authSvc *auth.Service, regSvc *registration.Service, connSvc *connection.Service, modsSvc *mods.Service, session notify.DiscordSession) *Handler {
	return newHandlerWithProvider(db, authSvc, nil, regSvc, connSvc, modsSvc, notify.NewDiscordProvider(session))
}

func newHandlerWithProvider(db *sql.DB, authSvc *auth.Service, bot *discord.Bot, regSvc *registration.Service, connSvc *connection.Service, modsSvc *mods.Service, provider notify.Provider) *Handler {
	return &Handler{
		db:            db,
		auth:          authSvc,
		registration:  regSvc,
		connection:    connSvc,
		mods:          modsSvc,
		discord:       bot,
		notifications: notifications.NewService(db),
		dispatcher: notify.NewDispatcher(db, map[string]notify.Provider{
			"discord": provider,
		}),
	}
}

func (h *Handler) AuthService() *auth.Service {
	return h.auth
}
