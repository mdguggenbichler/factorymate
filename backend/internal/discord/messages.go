package discord

import "fmt"

const helpMessageTemplate = `🏭 FactoryMate — quick start

**New here?**
1. /register — get a DM with a dashboard link (Discord sign-in, no password)
2. Finish registration on the web: choose username + in-game name
3. /mods export — download SMM profile → import in Satisfactory Mod Manager
4. /connection get — get server host, port, and password (sent to your DMs)
5. %s

**Already registered?**
/connection get — join details (DM)
/mods — full mod list
/set-player — fix your in-game name mapping
/clear-player — remove your in-game player mapping
/whoami — check your account status

**Have a web account (setup or invite)?**
Link Discord from **Account** on the dashboard after signing in with your password.

**Admins**
/connection set — update join details (broadcasts to all players)
/register-user — DM someone a registration link
/registration auto-approve — toggle approval gate

%s`

func formatHelpMessage() string {
	loginLine := "Log in via the dashboard (ask an admin for the URL)."
	dashboardLine := ""
	if url := PublicURL(); url != "" {
		loginLine = fmt.Sprintf("Log in: %s (Continue with Discord or password)", url+"/login")
		dashboardLine = fmt.Sprintf("Dashboard: %s", url)
	}
	return fmt.Sprintf(helpMessageTemplate, loginLine, dashboardLine)
}

func formatRegistrationPendingMessage(playerName string) string {
	return fmt.Sprintf("⏳ **Registration submitted**\n\nYour request is waiting for admin approval.\nYou'll receive a DM when approved with dashboard access and connection details.\n\nIn-game name claimed: %s", playerName)
}

func formatRegistrationApprovedMessage(username, role, playerLine string) string {
	dashboard := "the dashboard"
	if url := PublicURL(); url != "" {
		dashboard = url
	}
	return fmt.Sprintf("✅ **You're registered!**\n\nDashboard: %s\nUsername:  %s\nRole:      %s\nPlayer:    %s\n\nWe'll link your player automatically when you join.\nUse /set-player to correct your in-game name, or /clear-player to remove your mapping.\nUse /connection get for join details. Use /mods for the full mod list.",
		dashboard, username, role, playerLine)
}

func formatWelcomeApprovedDM(username string) string {
	dashboard := "the dashboard"
	if url := PublicURL(); url != "" {
		dashboard = url
	}
	return fmt.Sprintf("✅ **Registration approved!**\n\nWelcome to FactoryMate.\nDashboard: %s\nUsername: %s\nSign in with **Continue with Discord** or your password if you set one.", dashboard, username)
}

func formatRegistrationDeclinedDM(comment string) string {
	if comment != "" {
		return fmt.Sprintf("❌ **Registration declined**\n\n%s", comment)
	}
	return "❌ **Registration declined**\n\nYour registration request was not approved."
}

func playerDisplayLine(userPlayerName *string, pendingName string, linked bool) string {
	if userPlayerName != nil && *userPlayerName != "" {
		return *userPlayerName
	}
	if linked {
		return pendingName
	}
	if pendingName != "" {
		return pendingName + " (pending — not seen on server yet)"
	}
	return "—"
}

func dashboardURL(path string) string {
	base := PublicURL()
	if base == "" {
		return ""
	}
	return base + path
}
