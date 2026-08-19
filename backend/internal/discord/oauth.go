package discord

import (
	"context"
	"fmt"

	"factorymate/internal/auth"
)

func buildRegisterOAuthURL(ctx context.Context, authSvc *auth.Service, meta auth.OAuthStateMeta) (string, error) {
	if !auth.OAuthConfigured() {
		return "", auth.ErrOAuthNotConfigured
	}
	token, err := authSvc.CreateOAuthState(ctx, auth.OAuthPurposeRegister, meta)
	if err != nil {
		return "", err
	}
	return authSvc.BuildOAuthAuthorizeURL(ctx, token)
}

func formatRegisterOAuthMessage(oauthURL string) string {
	if oauthURL == "" {
		return "Discord sign-in is not configured on this server. Ask an admin to set DISCORD_CLIENT_SECRET and FACTORYMATE_PUBLIC_URL."
	}
	return fmt.Sprintf(
		"Finish your FactoryMate registration on the dashboard (Discord sign-in, no password):\n%s\n\nThe link expires in 10 minutes.",
		oauthURL,
	)
}
