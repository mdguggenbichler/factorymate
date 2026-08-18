# Developer Portal setup

FactoryMate uses a **single Discord bot** for channel notifications, direct messages, and slash commands. You do not create webhook URLs.

## 1. Create a Discord application

1. Open the [Discord Developer Portal](https://discord.com/developers/applications).
2. Click **New Application** and name it (e.g. `FactoryMate`).
3. Open the **Bot** tab → **Add Bot** → confirm.

## 2. Copy the bot token

1. Under **Bot** → **Token**, click **Reset Token** (or **Copy** if shown).
2. Store the token securely — you will set it as `DISCORD_BOT_TOKEN` in `.env`.

!!! warning "Keep the token secret"
    Never commit the token to git or share it publicly. Rotate it in the Developer Portal if leaked.

## 3. Set bot display name and avatar

On the **Bot** tab, set:

- **Username** — how the bot appears in Discord (e.g. `FactoryMate`)
- **Avatar** — optional icon for channel posts and DMs

Use the FactoryMate brand avatar for a consistent look:

- **Source file:** `assets/icon_full_bg_blue.png` in the FactoryMate repository
- **From a running instance:** open `/discord-bot-avatar.png` in your browser (Settings → Discord also shows a preview)
- **Upload:** Discord Developer Portal → your application → **Bot** → **Avatar**

This replaces the old per-webhook username/avatar overrides. One bot identity is used for all notifications.

## 4. Required bot permissions

When generating the invite URL (or in **OAuth2 → URL Generator**), enable these **bot permissions**:

| Permission | Why |
| --- | --- |
| View Channels | See channels you assign as notification targets |
| Send Messages | Post game-event embeds |
| Embed Links | Rich embed notifications |
| Use Slash Commands | `/register`, `/connection`, `/mods`, etc. |
| Send Messages in Threads | Post in thread targets if configured |
| Create Private Channels | Direct messages to users |

**Message Content Intent** is **not** required — FactoryMate uses slash commands and embeds only.

On the **Bot** tab, under **Privileged Gateway Intents**, enable **Server Members Intent**. FactoryMate needs this for role-based permissions and auto-linking Discord members to players. Without it, the gateway closes with `4014: Disallowed intent(s)` and the bot cannot connect.

## 5. OAuth2 scopes

In **OAuth2 → URL Generator**, select:

- `bot`
- `applications.commands`

You can generate a URL here for testing, but the recommended path is to use the invite URL from **Settings → Discord** in the FactoryMate dashboard (it includes the correct scopes and permissions).

## 6. Add to .env

```bash
DISCORD_BOT_TOKEN=your_token_here
DISCORD_GUILD_ID=your_server_id_here
```

To find your guild (server) ID: enable **Developer Mode** in Discord (Settings → Advanced), then right-click your server icon → **Copy Server ID**.

Optional fallback before UI role mapping is configured:

```bash
DISCORD_ADMIN_ROLE_IDS=123456789,987654321
```

## 7. Restart FactoryMate

After setting the token:

```bash
docker compose up -d
```

Bot features start automatically when `DISCORD_BOT_TOKEN` is set. Without it, the dashboard shows a warning under **Settings → Discord**.

## Next steps

1. Complete [First run](../first-run.md) — create your admin account first.
2. Use [Configuration](configuration.md) — invite the bot via the dashboard and map Discord roles.
3. Configure [notification targets](../managing/notifications.md).
