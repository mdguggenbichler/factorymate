# Discord configuration

After creating the bot in the [Developer Portal](setup.md), configure FactoryMate to connect it to your server.

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `DISCORD_BOT_TOKEN` | For Discord features | Bot token from Developer Portal |
| `DISCORD_GUILD_ID` | Recommended | Bootstrap guild ID until set in Settings → Discord |
| `DISCORD_ADMIN_ROLE_IDS` | Optional | Comma-separated admin role IDs before UI role mapping exists |
| `FACTORYMATE_PUBLIC_URL` | Optional | Public dashboard URL used in `/help` and welcome DMs |

Without `DISCORD_BOT_TOKEN`, the web dashboard and FRM polling still run. Bot commands, channel notifications, and DMs are disabled, and **Settings → Discord** shows a warning.

`DISCORD_GUILD_ID` in `.env` is used only when no guild ID is saved in the database yet. After you set the guild in the UI, the UI value takes precedence.

## Recommended setup order

```
1. Discord Developer Portal → create application → copy bot token
2. .env → DISCORD_BOT_TOKEN, DISCORD_GUILD_ID
3. docker compose up -d
4. Web UI → /setup → create first admin account
5. Settings → Discord → invite bot → configure role mappings
6. Settings → Notifications → Targets → pick a channel
7. Settings → Notifications → Templates → enable and assign message types
8. Players run /register in Discord
```

## Settings → Discord

Open **Settings → Discord** in the dashboard (admin only).

### Bot status

- **Connected** — bot token is set and the gateway is running.
- **Token not configured** — set `DISCORD_BOT_TOKEN` in `.env` and restart the container.

### Invite the bot

1. Click **Copy invite URL** (or open the link shown).
2. Open the URL in your browser and select your Discord server.
3. Confirm the bot has the permissions listed in [Developer Portal setup](setup.md).

The dashboard-generated invite URL includes scopes `bot` and `applications.commands` with the correct permission bitmask.

### Guild ID

Set your Discord server ID here if it was not bootstrapped from `DISCORD_GUILD_ID`. Slash commands register per guild when the bot starts — restart the backend after changing the guild ID.

### Role mappings

Map Discord roles to FactoryMate permissions:

| FactoryMate permission | Controls |
| --- | --- |
| Register | Who can run `/register` |
| Admin commands | `/connection set`, `/registration auto-approve`, etc. |

Until UI mappings exist, `DISCORD_ADMIN_ROLE_IDS` can grant admin command access.

### Auto-approve registrations

When **on** (default), new `/register` users are active immediately. When **off**, admins must approve registrations before users get dashboard access or connection credentials.

Configure here or via the Discord command `/registration auto-approve on|off`.

## After configuration

1. Go to [Notifications](../managing/notifications.md) — pick channels for game events.
2. Share [slash commands](commands.md) with your group — `/register` is the primary onboarding path.
