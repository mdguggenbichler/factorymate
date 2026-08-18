# Slash commands

FactoryMate registers slash commands in your Discord guild when the bot starts. Commands require the bot to be online and the user to meet permission and registration requirements.

## Who can use what

| User state | `/register` | `/connection`, `/mods` | Admin commands |
| --- | --- | --- | --- |
| Unregistered member | If role allows | No | No |
| Pending approval | No | No | No |
| Active, Discord linked | No (already registered) | Yes | If admin role |
| Active, not linked | No | No | No — use `/link` |

## Core commands (all users)

| Command | Permission | Description |
| --- | --- | --- |
| `/help` | Anyone | Command list, dashboard URL, onboarding hints |
| `/whoami` | Anyone | Link status: FM username, role, mapped player, approval state |
| `/register` | Member with register permission; not already linked | Self-serve account; prompts for in-game name and dashboard password |
| `/link` | Discord user not yet linked; has existing FM account | Attach Discord to an existing web account |
| `/set-player <name>` | Active registered user | Update in-game player mapping |
| `/connection get` | Active registered user | DM current game join details |
| `/mods` | Active registered user | `list` (default): mod table; `export`: SMM profile download |

## Admin commands

| Command | Permission | Description |
| --- | --- | --- |
| `/register-user` | Admin | DM target user to complete registration (always auto-approved) |
| `/connection set` | Admin | Set join host/port/password; DMs all active linked players |
| `/registration auto-approve` | Admin | Toggle auto-approve (`on` / `off`) |
| `/registrations list` | Admin | Pending approval queue |
| `/registrations approve\|reject` | Admin | Approve or reject a pending registration |
| `/status` | Active registered user | Server online, player count (from FRM cache) |
| `/players` | Active registered user | Who is online |
| `/unlink @user` | Admin | Remove Discord link (keeps FM account) |
| `/broadcast <message>` | Admin | DM all registered players |
| `/sync-roles` | Admin | Re-apply Discord → FM role mapping |
| `/password-reset @user` | Admin | Trigger password reset flow |
| `/notifications` | Active registered user | View/toggle DM notification preferences by category |

## Registration flow

Primary onboarding for new players:

1. User runs `/register` in Discord.
2. Bot opens a modal: in-game username (required) and dashboard password (required, minimum 8 characters).
3. FactoryMate creates the account, links the Discord identity, and saves the in-game name.
4. If auto-approve is on, the user can log in and use `/connection` and `/mods` immediately.
5. If auto-approve is off, an admin must approve before access is granted.

Username is derived from the Discord display name with automatic deduplication (`alex`, `alex-2`, …).

## Connection details

- **`/connection get`** — sends join details (host, port, optional password) via DM as an embed.
- **`/connection set`** (admin) — updates join details and broadcasts a DM to all active linked players.

Configure default join details in **Settings → Connection** on the web dashboard.

## Mod list

- **`/mods list`** — embed with installed mods, game build, and SML version.
- **`/mods export`** — attaches an `.smmprofile` file for Satisfactory Mod Manager.

The same mod list is available on the web at `/mods`.

## Troubleshooting

| Problem | Check |
| --- | --- |
| Commands not visible | Bot invited with `applications.commands` scope; restart after guild ID change |
| `/register` rejected | User may lack register role mapping; may already be linked |
| No DMs received | User must allow DMs from server members; bot needs **Create Private Channels** |
| Bot appears offline | Verify `DISCORD_BOT_TOKEN`; check container logs |
