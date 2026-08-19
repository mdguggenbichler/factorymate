# Slash commands

FactoryMate registers slash commands in your Discord guild when the bot starts **and** when an admin saves a new guild ID in **Settings → Discord** (no container restart required).

## Who can use what

| User state | `/register` | `/connection`, `/mods` | Admin commands |
| --- | --- | --- | --- |
| Unregistered member | If role allows | No | No |
| Pending approval | No | No | No |
| Active, Discord linked | No (already registered) | Yes | If admin role |
| Active, not linked | No | No | No — link Discord from **Account** on the web |

## Core commands (all users)

| Command | Permission | Description |
| --- | --- | --- |
| `/help` | Anyone | Command list, dashboard URL, onboarding hints |
| `/whoami` | Anyone | Link status: FM username, role, mapped player, approval state |
| `/register` | Member with register permission; not already linked | DM with OAuth link to finish registration on the web (Discord sign-in, no password) |
| `/set-player <name>` | Active registered user | Update in-game player mapping |
| `/clear-player` | Active registered user | Remove in-game player mapping |
| `/connection get` | Active registered user | DM current game join details |
| `/mods` | Active registered user | `list` (default): mod table; `export`: SMM profile download |

## Admin commands

| Command | Permission | Description |
| --- | --- | --- |
| `/register-user` | Admin | DM target user an OAuth registration link (always auto-approved) |
| `/connection set` | Admin | Set join host/port/password; DMs all active linked players |
| `/registration auto-approve` | Admin | Toggle auto-approve (`on` / `off`) |
| `/registrations list` | Admin | Pending approval queue |
| `/registrations approve\|reject` | Admin | Approve or reject a pending registration |
| `/status` | Active registered user | Server online, player count (from FRM cache) |
| `/players` | Active registered user | Who is online |
| `/unlink @user` | Admin | Remove Discord link (keeps FM account) |
| `/broadcast <message>` | Admin | DM all registered players |
| `/sync-roles` | Admin | Re-apply Discord → FM role mapping |
| `/password-reset @user` | Admin | Points admin to web **Settings → Users** (no DM temp password) |
| `/notifications` | Active registered user | View/toggle DM notification preferences by category |

## Registration flow

Primary onboarding for new players:

1. User runs `/register` in Discord.
2. Bot DMs an OAuth link (or shows it ephemerally if DMs are blocked).
3. User authorizes Discord (`identify` scope only).
4. Web form at `/register/complete`: choose dashboard username and in-game player name — **no password**.
5. If auto-approve is on, the user can sign in with **Continue with Discord** immediately.
6. If auto-approve is off, an admin must approve before access is granted.

**Setup admin and break-glass invite users:** sign in with username/password, then **Account → Link Discord** (no `/link` slash command).

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
| Commands not visible | Bot invited with `applications.commands` scope; save guild ID in Settings → Discord |
| `/register` rejected | User may lack register role mapping; may already be linked |
| OAuth link missing | Set `DISCORD_CLIENT_SECRET` and `FACTORYMATE_PUBLIC_URL`; add OAuth redirect in Developer Portal |
| No DMs received | User must allow DMs from server members; bot needs **Create Private Channels** |
| Bot appears offline | Verify `DISCORD_BOT_TOKEN`; check container logs |
