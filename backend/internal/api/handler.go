package api

import (
	"database/sql"

	"factorymate/internal/auth"
	"factorymate/internal/discord"
	"factorymate/internal/notify"
)

type Handler struct {
	db         *sql.DB
	auth       *auth.Service
	dispatcher *notify.Dispatcher
	discord    *discord.Bot
}

func NewHandler(db *sql.DB, authSvc *auth.Service, bot *discord.Bot) *Handler {
	var session notify.DiscordSession
	if bot != nil {
		session = bot.Session()
	}
	return newHandler(db, authSvc, bot, session)
}

// NewHandlerWithDiscordSession is for tests that need a mock Discord session without a live bot.
func NewHandlerWithDiscordSession(db *sql.DB, authSvc *auth.Service, session notify.DiscordSession) *Handler {
	return newHandler(db, authSvc, nil, session)
}

func newHandler(db *sql.DB, authSvc *auth.Service, bot *discord.Bot, session notify.DiscordSession) *Handler {
	provider := notify.NewDiscordProvider(session)
	return &Handler{
		db:      db,
		auth:    authSvc,
		discord: bot,
		dispatcher: notify.NewDispatcher(db, map[string]notify.Provider{
			"discord": provider,
		}),
	}
}

func (h *Handler) AuthService() *auth.Service {
	return h.auth
}
