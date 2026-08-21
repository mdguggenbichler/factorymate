package api

import (
	"context"
	"database/sql"

	"factorymate/internal/auth"
	"factorymate/internal/connection"
	"factorymate/internal/discord"
	"factorymate/internal/mods"
	"factorymate/internal/notifications"
	"factorymate/internal/notify"
	"factorymate/internal/planner"
	"factorymate/internal/registration"
)

// DiscordBot is the bot surface used by admin settings and registration DMs.
type DiscordBot interface {
	Connected() bool
	ClearSlashCommands(ctx context.Context, guildID string) error
	RegisterSlashCommands(ctx context.Context) error
	ListGuildTextChannels(ctx context.Context) ([]discord.Channel, error)
	InviteURL() (string, error)
	SendWelcomeDM(ctx context.Context, externalUserID, username string)
	SendRegistrationDeclinedDM(ctx context.Context, externalUserID, comment string)
}

var _ DiscordBot = (*discord.Bot)(nil)

type Handler struct {
	db            *sql.DB
	auth          *auth.Service
	registration  *registration.Service
	connection    *connection.Service
	mods          *mods.Service
	dispatcher    *notify.Dispatcher
	discord       DiscordBot
	notifications *notifications.Service
	planner       *planner.Catalog
	plannerIcons  string
	oauthExchange func(context.Context, string) (auth.DiscordUserResponse, error)
}

func NewHandler(db *sql.DB, authSvc *auth.Service, bot *discord.Bot, regSvc *registration.Service, connSvc *connection.Service, modsSvc *mods.Service, plannerCat *planner.Catalog, plannerCfg planner.Config) *Handler {
	return newHandlerWithProvider(db, authSvc, bot, regSvc, connSvc, modsSvc, plannerCat, plannerCfg, notify.NewDiscordProviderWithSessionFn(func() notify.DiscordSession {
		if bot == nil {
			return nil
		}
		return bot.Session()
	}))
}

// NewHandlerWithDiscordSession is for tests that need a mock Discord session without a live bot.
func NewHandlerWithDiscordSession(db *sql.DB, authSvc *auth.Service, regSvc *registration.Service, connSvc *connection.Service, modsSvc *mods.Service, plannerCat *planner.Catalog, plannerCfg planner.Config, session notify.DiscordSession) *Handler {
	return newHandlerWithProvider(db, authSvc, nil, regSvc, connSvc, modsSvc, plannerCat, plannerCfg, notify.NewDiscordProvider(session))
}

func newHandlerWithProvider(db *sql.DB, authSvc *auth.Service, bot *discord.Bot, regSvc *registration.Service, connSvc *connection.Service, modsSvc *mods.Service, plannerCat *planner.Catalog, plannerCfg planner.Config, provider notify.Provider) *Handler {
	var discordBot DiscordBot
	if bot != nil {
		discordBot = bot
	}
	return &Handler{
		db:            db,
		auth:          authSvc,
		registration:  regSvc,
		connection:    connSvc,
		mods:          modsSvc,
		discord:       discordBot,
		notifications: notifications.NewService(db),
		planner:       plannerCat,
		plannerIcons:  plannerCfg.IconsDir,
		dispatcher: notify.NewDispatcher(db, map[string]notify.Provider{
			"discord": provider,
		}),
	}
}

func (h *Handler) AuthService() *auth.Service {
	return h.auth
}

// SetDiscordBot replaces the Discord bot used by settings handlers (tests).
func (h *Handler) SetDiscordBot(bot DiscordBot) {
	h.discord = bot
}

// SetOAuthCodeExchange replaces Discord token exchange (tests).
func (h *Handler) SetOAuthCodeExchange(fn func(context.Context, string) (auth.DiscordUserResponse, error)) {
	h.oauthExchange = fn
}
