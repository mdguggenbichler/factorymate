package api

import (
	"database/sql"

	"factorymate/internal/auth"
	"factorymate/internal/notify"
)

type Handler struct {
	db         *sql.DB
	auth       *auth.Service
	dispatcher *notify.Dispatcher
}

func NewHandler(db *sql.DB, authSvc *auth.Service) *Handler {
	return &Handler{
		db:   db,
		auth: authSvc,
		dispatcher: notify.NewDispatcher(db, map[string]notify.Provider{
			"discord": notify.NewDiscordProvider(),
		}),
	}
}

func (h *Handler) AuthService() *auth.Service {
	return h.auth
}
