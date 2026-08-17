package api

import (
	"github.com/go-chi/chi/v5"
)

func (h *Handler) Mount(r chi.Router) {
	r.Post("/auth/setup", h.Setup)
	r.Post("/auth/login", h.Login)
	r.Get("/invites/{token}", h.GetInvite)
	r.Post("/invites/{token}/accept", h.AcceptInvite)

	r.Group(func(r chi.Router) {
		r.Use(h.auth.RequireSession(writeError))

		r.Post("/auth/logout", h.Logout)
		r.Get("/auth/me", h.Me)

		r.Group(func(r chi.Router) {
			r.Use(h.auth.RequireActiveUser(writeError))

			r.Put("/account/password", h.ChangePassword)

			r.Get("/status", h.GetStatus)
			r.Get("/players", h.GetPlayers)
			r.Get("/players/history", h.GetPlayersHistory)
			r.Get("/power", h.GetPower)
			r.Get("/power/history", h.GetPowerHistory)
			r.Get("/power/metrics", h.GetPowerMetrics)
			r.Get("/production", h.GetProduction)
			r.Get("/production/items", h.GetProductionItems)
			r.Get("/production/current", h.GetProductionCurrent)
			r.Get("/production/machines", h.GetProductionMachines)
			r.Get("/resource-sink", h.GetResourceSink)
			r.Get("/resource-sink/history", h.GetResourceSinkHistory)
			r.Get("/drones", h.GetDrones)
			r.Get("/doggos", h.GetDoggos)
			r.Get("/milestones", h.GetMilestones)
			r.Get("/research", h.GetResearch)
			r.Get("/vehicles", h.GetVehicles)
			r.Get("/elevator", h.GetElevator)

			r.Get("/mods", h.GetMods)
			r.Get("/mods/smmprofile", h.GetModsSMMProfile)
			r.Get("/connection-details", h.GetConnectionDetails)

			r.Group(func(r chi.Router) {
				r.Use(h.auth.RequireAdmin(writeError))

				r.Post("/mods/refresh", h.PostModsRefresh)
				r.Put("/connection-details", h.PutConnectionDetails)

				r.Get("/elevator/unknown-log", h.GetElevatorUnknownLog)
				r.Post("/elevator/unknown-log/{id}/resolve", h.ResolveElevatorUnknownLog)

				r.Get("/notification-targets", h.ListNotificationTargets)
				r.Post("/notification-targets", h.CreateNotificationTarget)
				r.Put("/notification-targets/{id}", h.UpdateNotificationTarget)
				r.Delete("/notification-targets/{id}", h.DeleteNotificationTarget)
				r.Post("/notification-targets/{id}/test", h.TestNotificationTarget)

				r.Get("/message-types", h.ListMessageTypes)
				r.Put("/message-types/{key}/enabled", h.UpdateMessageTypeEnabled)
				r.Put("/message-types/{key}/template", h.UpdateMessageTypeTemplate)
				r.Post("/message-types/{key}/template/reset", h.ResetMessageTypeTemplate)
				r.Post("/message-types/{key}/template/preview", h.PreviewMessageTypeTemplate)
				r.Post("/message-types/{key}/template/test", h.TestMessageTypeTemplate)
				r.Put("/message-types/{key}/targets", h.UpdateMessageTypeTargets)

				r.Get("/notification-log", h.GetNotificationLog)
				r.Get("/settings", h.GetSettings)
				r.Put("/settings", h.UpdateSettings)
				r.Post("/settings/frm/test", h.TestFRMConnection)
				r.Get("/users", h.ListUsers)
				r.Put("/users/{id}", h.UpdateUser)
				r.Delete("/users/{id}", h.DeleteUser)
				r.Post("/invites", h.CreateInvite)
				r.Get("/invites", h.ListInvites)
				r.Delete("/invites/{id}", h.RevokeInvite)

				r.Get("/discord/settings", h.GetDiscordSettings)
				r.Put("/discord/settings", h.UpdateDiscordSettings)
				r.Get("/discord/channels", h.ListDiscordChannels)
				r.Get("/discord/invite-url", h.GetDiscordInviteURL)

				r.Get("/registrations/pending", h.ListPendingRegistrations)
				r.Post("/registrations/{id}/approve", h.ApproveRegistration)
				r.Post("/registrations/{id}/reject", h.RejectRegistration)
				r.Get("/players/unmapped", h.ListUnmappedPlayers)
				r.Put("/users/{id}/external", h.UpdateUserExternal)
			})
		})
	})
}
