package auth

import (
	"net/http"
)

type errorWriter func(w http.ResponseWriter, r *http.Request, status int, message string)

func (s *Service) RequireSession(writeError errorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, user, err := s.SessionFromRequest(r.Context(), r)
			if err != nil {
				ClearSessionCookie(w, r)
				writeError(w, r, http.StatusUnauthorized, "unauthenticated")
				return
			}
			ctx := withUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (s *Service) RequireActiveUser(writeError errorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "unauthenticated")
				return
			}
			if user.Status == StatusPendingApproval {
				writeError(w, r, http.StatusForbidden, "account pending approval")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Service) RequireAdmin(writeError errorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "unauthenticated")
				return
			}
			if user.Role != RoleAdmin {
				writeError(w, r, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
