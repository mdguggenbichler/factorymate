package discord

import "fmt"

const helpMessageTemplate = `🏭 FactoryMate — quick start

**New here?**
1. /register — create your dashboard account
2. /mods export — download SMM profile → import in Satisfactory Mod Manager
3. /connection — get server host, port, and password (sent to your DMs)
4. Log in: %s

**Already registered?**
/connection — join details (DM)
/mods — full mod list
/set-player — fix your in-game name mapping
/whoami — check your link status

**Have a web account but new to Discord?**
/link — attach Discord to your existing login

**Admins**
/connection set — update join details (broadcasts to all players)
/register user — invite someone to complete registration
/registration auto-approve — toggle approval gate

Dashboard: %s`

func formatHelpMessage() string {
	url := PublicURL()
	return fmt.Sprintf(helpMessageTemplate, url, url)
}

func formatRegistrationPendingMessage(playerName string) string {
	return fmt.Sprintf("⏳ **Registration submitted**\n\nYour request is waiting for admin approval.\nYou'll receive a DM when approved with dashboard access and connection details.\n\nIn-game name claimed: %s", playerName)
}

func formatRegistrationApprovedMessage(username, role, playerLine string) string {
	return fmt.Sprintf("✅ **You're registered!**\n\nDashboard: %s\nUsername:  %s\nRole:      %s\nPlayer:    %s\n\nWe'll link your player automatically when you join.\nUse /set-player to correct your in-game name.\nUse /connection for join details. Use /mods for the full mod list.",
		PublicURL(), username, role, playerLine)
}

func formatWelcomeApprovedDM(username string) string {
	return fmt.Sprintf("✅ **Registration approved!**\n\nWelcome to FactoryMate.\nDashboard: %s\nUsername: %s", PublicURL(), username)
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
