# First run

After [installing FactoryMate](installation/docker-compose.md) and starting the container, complete these steps once.

## 1. Create the admin account

Open `http://<your-host>:3000`.

If no users exist, you are redirected to the **one-time admin setup** page (`/setup`).

1. Choose an admin username and password.
2. Log in to the dashboard.

This step must happen before or alongside Discord setup. The first admin is created via the web UI only.

## 2. Verify FRM connectivity

Go to **Settings → General**.

1. Confirm **FRM host** and **FRM port** match your [FRM connectivity](frm-connectivity.md) setup.
2. Check that the dashboard shows live data (players, power, etc.).

If FRM is unreachable, the poller logs errors and the dashboard stays empty until host/port are correct.

## 3. Configure the Discord bot

Ensure `DISCORD_BOT_TOKEN` is set in `.env` and the container has been restarted.

Go to **Settings → Discord**:

1. Confirm bot status shows **Connected**.
2. Open the **invite URL** and add the bot to your Discord server.
3. Set the **guild ID** if not already bootstrapped from `DISCORD_GUILD_ID`.
4. Configure **role mappings** — who can `/register` and who has admin commands.
5. Leave **auto-approve** on unless you want manual approval for new registrations.

See [Discord setup](discord/setup.md) and [Configuration](discord/configuration.md) for details.

## 4. Add a notification target

Go to **Settings → Notifications → Targets**.

1. Click **Add target**.
2. Name the target (e.g. `factory-alerts`).
3. Pick a **Discord channel** from the dropdown (requires the bot to be in the guild).
4. Save.

If you upgraded from an older version with webhook targets, re-select channels — webhook URLs are no longer supported.

## 5. Enable message types

Go to **Settings → Notifications → Templates**.

1. Enable message types you care about (e.g. `player_joined`, `player_left`, `fuse_tripped`, `server_offline`).
2. Assign each enabled type to your notification target.
3. Use **Send test** on a template to verify embeds appear in Discord.

## 6. Onboard players

Ask new group members to run **`/register`** in your Discord server. This is the primary onboarding path.

Optional break-glass recovery: **Settings → Users → Advanced** — create a single-use web invite link if someone cannot use Discord.

## 7. Optional: connection details and mod list

- **Settings → Connection** — set game join host, port, and optional client password. Players can retrieve these with `/connection` in Discord.
- **`/mods`** in Discord or **`/mods`** on the web — view installed mods and export an SMM profile.

## Smoke checks

```bash
curl -s http://localhost:3000/healthz
# {"status":"ok"}
```

Manual checks:

- Dashboard loads with live FRM data.
- **Send test** on a notification template delivers an embed to your Discord channel.
- A test player can `/register` and log in to the dashboard.
- `docker compose restart` — settings and history persist in `./data`.

After first run, see [Managing settings](managing/settings.md) for day-to-day administration.
