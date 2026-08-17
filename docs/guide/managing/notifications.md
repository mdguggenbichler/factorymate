# Notifications

FactoryMate sends game-event alerts as Discord embeds to channels you configure. The same bot also handles optional direct messages (see per-user preferences).

## How it works

```
FRM poll → diff engine detects change → template renderer → Discord bot → channel post
```

Message types (e.g. `player_joined`, `fuse_tripped`, `server_offline`) are defined in the system. You control which fire, what they say, and where they go.

## Notification targets

**Settings → Notifications → Targets**

A target is a named Discord channel assignment:

1. Click **Add target**.
2. Enter a name.
3. Select a **channel** from the dropdown (populated via the bot API).
4. Optionally set a **thread ID** for thread-specific posts.
5. Save.

The bot must be in your guild and have permission to post in the selected channel.

### Legacy webhook targets

If you upgraded from a version that used webhook URLs, existing targets show a legacy badge. Re-select a Discord channel for each target — webhook URLs are not supported in current releases.

### Send test

Each target has a **Send test** action that posts a sample embed to verify channel permissions and bot connectivity.

## Message templates

**Settings → Notifications → Templates**

For each message type:

| Control | Purpose |
| --- | --- |
| **Enabled** | Turn this event type on or off |
| **Target assignment** | Which notification target receives posts |
| **Template editor** | Customize title, description, color, fields, footer |
| **Send test** | Fire a sample notification for this type |

Default embed copy ships with FactoryMate. Custom edits saved in the UI override defaults. Use **Reset to default** after upgrades to pick up improved default copy.

### Common message types

| Type | When it fires |
| --- | --- |
| `player_joined` | A player connects to the server |
| `player_left` | A player disconnects |
| `fuse_tripped` | A fuse blows in the factory |
| `server_offline` | FRM becomes unreachable |
| `server_online` | FRM comes back after being offline |
| `schematic_purchased` | A milestone or schematic is unlocked |
| `train_derailed` | A train derails |

See the full list in the Templates UI.

## Admin DM defaults

**Settings → Notifications → Defaults**

Set default category toggles for new users' DM preferences (M16). Categories group related events (power, players, milestones, etc.).

## Per-user DM preferences

**Account → Notifications** (each user)

Users with linked Discord accounts can opt in or out of DM categories. Personal player join/leave DMs are a separate toggle.

Connection-detail broadcasts always DM all active linked players regardless of category prefs.

## Notification log

**Settings → Notifications → Log**

Audit trail of sent notifications: message type, target, timestamp, and delivery status. Useful for debugging missed alerts or Discord API errors.

Rows referencing deleted targets show "Deleted target" rather than failing.

## Troubleshooting

| Symptom | Things to check |
| --- | --- |
| No channel posts | Message type enabled? Target assigned? Bot online and in channel? |
| Test send fails | Bot permissions in target channel; guild ID correct |
| Duplicate messages | Usually indicates two enabled assignments or poller restart edge — check log |
| Embeds look wrong | Edit template or reset to default |

Bot token and channel configuration: [Discord configuration](../discord/configuration.md).
