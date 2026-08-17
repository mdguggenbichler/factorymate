# Settings overview

The web dashboard **Settings** section (admin-only except where noted) controls FactoryMate behavior. Most values persist in SQLite and survive container restarts.

## Settings → General

**Path:** `/settings/general`

| Setting | Purpose |
| --- | --- |
| FRM host / port | Where FactoryMate polls game state |
| Poll interval | How often FRM is polled (seconds) |
| Server display name | Synced from FRM; shown in dashboard and notifications |

Initial FRM host/port can be seeded from `FRM_HOST` and `FRM_PORT` in `.env` on first boot.

## Settings → Discord

**Path:** `/settings/discord`

Bot status, invite URL, guild ID, Discord role → permission mappings, and auto-approve toggle.

See [Discord configuration](../discord/configuration.md).

## Settings → Connection

**Path:** `/settings/connection`

Game join details separate from FRM monitoring:

- Join host and port
- Optional client password
- SMM profile name for mod export

When connection details change (here or via `/connection set`), all active linked players receive a DM.

Viewers can read connection details on the dashboard; only admins can edit.

## Settings → Notifications

| Page | Path | Purpose |
| --- | --- | --- |
| Targets | `/settings/notifications/targets` | Discord channels for game-event posts |
| Templates | `/settings/notifications/templates` | Enable message types, edit embeds, assign targets |
| Defaults | `/settings/notifications/defaults` | Admin defaults for per-user DM preferences |
| Log | `/settings/notifications/log` | History of sent notifications |

See [Notifications](notifications.md).

## Settings → Users

**Path:** `/settings/users`

- View all users, roles, Discord link status, and player mapping
- Pending approval queue (when auto-approve is off)
- Unmapped players (in-game names without linked accounts)
- Break-glass web invite links (Advanced section)

See [Users & registration](users.md).

## Account → Notifications

**Path:** `/account/notifications` (all active users)

Per-user toggles for DM notification categories. New users inherit admin defaults from **Settings → Notifications → Defaults**.

## Dashboard pages (non-settings)

| Page | Path | Access |
| --- | --- | --- |
| Players | `/players` | All authenticated users |
| Power | `/power` | All authenticated users |
| Production | `/production` | All authenticated users |
| Mods | `/mods` | All authenticated users |
| Milestones, research, vehicles, etc. | Various | All authenticated users |

## Roles

| Role | Capabilities |
| --- | --- |
| **admin** | Full settings access, user management, Discord admin commands |
| **viewer** | Dashboard read access, personal `/connection` and `/mods`, own notification prefs |

Discord role mappings can assign FM roles at registration time.
