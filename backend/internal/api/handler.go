package api

import "factorymate/internal/auth"

type Handler struct {
	auth *auth.Service
}

func NewHandler(authSvc *auth.Service) *Handler {
	return &Handler{auth: authSvc}
}

func (h *Handler) AuthService() *auth.Service {
	return h.auth
}
