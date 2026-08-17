# Requirements

Before installing FactoryMate, confirm your environment meets these requirements.

## Satisfactory dedicated server

| Component | Notes |
| --- | --- |
| Satisfactory dedicated server | Running with **Satisfactory Mod Loader (SML)** |
| FicsIt Remote Monitoring (FRM) | Installed and enabled on the server |
| FRM HTTP API | Reachable from the FactoryMate container over HTTP |

### FRM configuration

In-game (Server Manager or FRM config), ensure:

- **Web server autostart** is enabled (`Web_Autostart` or equivalent in FRM settings).
- FRM listens on a known host and port (commonly `:8080` inside the game container, often mapped to something like `:8889` on the host).

FactoryMate only uses FRM **Read** endpoints — it never modifies your save or factory.

## Discord

| Component | Notes |
| --- | --- |
| Discord server (guild) | Where your group plays and receives alerts |
| Discord application + bot | Created in the [Discord Developer Portal](discord/setup.md) |
| Bot token | Set as `DISCORD_BOT_TOKEN` in your `.env` file |

Without a bot token, the web dashboard and FRM polling still work, but **Discord notifications, slash commands, and DMs are disabled**.

## Docker

Docker and Docker Compose are required for the deployment paths documented here.

- **Docker Engine** 20.10+ recommended
- **Docker Compose** v2 (`docker compose` command)

FactoryMate runs as a single container exposing port `3000` (configurable via `FACTORYMATE_PORT`).

## Network

FactoryMate must reach:

| Destination | Protocol | Purpose |
| --- | --- | --- |
| FRM HTTP API | HTTP | Poll game state |
| Discord API | HTTPS | Bot gateway, channel posts, DMs |

Outbound internet access to `discord.com` is required when using the Discord bot.

## Optional but recommended

| Variable | Purpose |
| --- | --- |
| `FACTORYMATE_PUBLIC_URL` | Public dashboard URL shown in bot welcome messages and `/help` |
| `DISCORD_GUILD_ID` | Bootstrap guild ID before you configure Settings → Discord |

See [FRM connectivity](frm-connectivity.md) for how FactoryMate reaches FRM on your network.
