# Proposal: Inactive player cleanup (optional)

**Status:** possible future feature — not v1, not scheduled.  
**Related:** spec §3 `users` / `player_state`, spec §6 auth, discord-bot-plan §7–§8 (player mapping, connection details).

Dedicated-server communities accumulate Discord members and FactoryMate accounts for people who stopped playing. This feature would optionally warn them, then kick them from the Discord guild and mark the FactoryMate user inactive — **without deleting their FM data**, so an admin can restore access later.

Default for the whole feature: **off**.

---

## Goals

- Let an admin configure: “no in-game presence for **X** days → warn; if no confirmation within **Y** days → act.”
- After action:
  - Kick the member from the Discord guild (so they no longer sit in chat and cannot use bot commands / receive connection DMs as a guild member).
  - Set the FactoryMate user to **`inactive`**.
- **Keep** the FactoryMate user row, Discord identity fields, player mapping, notification prefs, and history. In-game `player_state` is poller data and is never deleted by this feature.
- **Reactivation is admin-only.** Silence after a warning is not proof they quit (illness, travel, missed Discord). The player cannot self-serve back to `active`.
- Inactive users must not receive **new join/connection details** (web API, dashboard “how to join”, `/connection`, connection-change DMs) or other player-facing bot chat that is gated on `status = active` today.

## Non-goals

- Deleting FactoryMate accounts, player history, or FRM snapshots.
- Banning the Discord user (kick only — they may re-join the guild later; they still need an admin to set FM `active`).
- Using Discord last-online or FactoryMate dashboard login as the inactivity clock (the clock is **in-game** last seen).
- Auto-kicking FactoryMate **admins**, Discord members mapped to admin roles, or an explicit exempt list.
- Shipping this in the current roadmap (M0–M13 / M14 backlog items already listed in spec §10).

---

## Why kick and inactive together

Connection details and player commands are already limited to `users.status = active` (discord-bot-plan §8). Kicking from Discord is the guild-side half: they leave the community chat and cannot run slash commands as a member.

Both are required for the intended outcome:

| Action | Effect |
|--------|--------|
| Kick from Discord | Not in the guild; no channel mentions, no `/connection`, no DMs that require being a member / linked guild user |
| FM `inactive` | Session/API/bot logic refuses player surfaces even if they re-join Discord or still have a cookie |

Keeping FM data means a returning player can be linked again without re-creating history or re-mapping the in-game ID from scratch. They still need an admin to flip `active` after the admin is satisfied they should be back.

---

## Inactivity signal

**Primary clock:** `player_state.last_seen_at` (set on leave when `Online` `true → false`; unchanged while online; `NULL` until first observed leave — spec §4.1.1).

A player who is **currently online** is never inactive.

**Must define separately (do not treat as “last seen X days ago”):**

- `last_seen_at` is `NULL` (joined but never observed leaving, or never fully ingested).
- User is registered / Discord-linked but **unmapped** (`player_id` unset, only `pending_player_name`) — never appeared in FRM `getPlayer`.
- FRM/poller was down for a long stretch (stale last-seen looks like inactivity).

Suggested default for the unmapped / never-seen case: **skip automatic action**; surface them on an admin preview list only. Optional later: a second clock from `users.created_at` for “registered, never played.”

---

## Proposed flow (when enabled)

1. **Detect** linked, non-exempt users whose mapped player has `last_seen_at` older than **X** days (and is not online).
2. **Warn** in a configured Discord channel: mention the Discord user, in-game name, last seen, and deadline. Prefer a persistent **“I'm still playing”** button (not a raw emoji reaction — reactions are easy to miss or apply by accident). Optional extra: DM the same prompt (DMs are often blocked; channel ping remains the fallback).
3. If they confirm within **Y** days: clear the warning; do not act. (Confirmation does not require an admin.)
4. If they do **not** confirm: **kick** from the guild (if that toggle is on) and set FM status to **`inactive`**. Record an audit row (who/what/when, last seen used, warning message id).
5. Later: they may re-join Discord and even `/register` or link again depending on implementation — **dashboard + connection + commands stay blocked** until an admin sets the user **active** in Settings → Users (or an admin slash command). Data is still there.

Manual path without automation: admin preview list → warn / mark inactive / kick with confirmation. Useful as an MVP before any cron.

### Independent toggles (all default off)

The parent feature being optional is not enough; kicking is more destructive than FM inactive:

1. Enable inactivity policy
2. Post warnings (channel id, mention)
3. Set FactoryMate `inactive` after grace period
4. Kick from Discord after grace period

Typical enablement: 1+2+3+4. An admin may want warn + inactive without kick, or preview-only (1 off, use the roster).

---

## FactoryMate `inactive` behavior

Today `users.status` is `pending_approval | active` (spec §3). Add **`inactive`**.

While `inactive`:

- Cannot log in to the dashboard (or: login allowed but all player routes 403 with a “account inactive — ask an admin” page — pick one in spec; **prefer block login** so it matches pending_approval).
- Excluded from connection-detail broadcasts and `/connection`.
- Excluded from personal player DMs and other `status = active` queries.
- Player mapping, `external_*` Discord fields, prefs, and history **retained**.
- Uniqueness of Discord identity remains: a new `/register` for the same Discord id should attach to the existing inactive row (or be rejected with “account exists, ask an admin”), **not** create a duplicate user.

**Reactivate:** admin only (`PUT /api/users/:id` status, and/or `/users activate @user`). No automatic reactivate on next in-game join (that would undo the “they didn’t press the button because they were sick” case without a human check — they might also be a different person on the same PC). Optional later: notify admins when an inactive mapped player comes online.

---

## Discord kick implications

- Bot needs **Kick Members**; bot role must be **above** targets. Cannot kick the guild owner or members with equal/higher roles.
- Do not kick FM admins or Discord roles mapped to admin.
- Kick is not a ban: they can re-join if the invite is public. FM stays `inactive` until an admin restores it.
- After kick, `external_*` may still store the Discord snowflake so the same person can be recognized on re-join; the guild membership is gone until they come back.

---

## Settings (sketch)

Admin UI, e.g. Settings → Discord or Settings → Users:

| Setting | Notes |
|---------|--------|
| Enabled | Master switch, default off |
| Inactivity threshold (X) | Days since `last_seen_at` |
| Grace period (Y) | Days after warning before action |
| Warning channel | Required if warnings on |
| Warn / set inactive / kick | Independent booleans |
| Exempt Discord role IDs | Plus implicit FM `role = admin` |
| Dry-run / preview | List who would be warned or acted on now |

i18n: all copy in `messages/en.json` (spec §8.2). Warning/DM templates should follow `{VarName}` templating if they are user-editable; otherwise fixed i18n strings.

---

## Suggested delivery (if scheduled later)

1. **Roster only:** inactive-by-last-seen list on Users or Players; no automation.
2. **Warn:** channel message + button + grace tracking.
3. **`inactive` status** + gate connection/chat/login + admin reactivate + keep data.
4. **Kick** as a separate toggle, audit, exempt roles, preview of kickable members.

Tests: httptest / mocked Discord (no live guild); fixtures for `last_seen_at` NULL vs stale vs online; never-kick-admin; inactive user omitted from connection broadcast.

---

## Open questions (leave for spec when this is scheduled)

- Login vs “logged in but frozen” page for `inactive`.
- Same Discord user re-joins the guild: auto-relink to the inactive FM row vs require `/register` again.
- Whether in-game join of an inactive mapped player should **notify admins** only.
- Second policy for “registered, never seen in FRM.”
- Store warning state (table vs JSON) and what happens if the warning message is deleted.
- Timezone / calendar day vs rolling 24h windows for X and Y.

---

## Out of scope for this note

Do not treat this file as spec. When the feature is scheduled, promote into `docs/factorymate-spec.md` (schema, API, settings) and `docs/discord-bot-plan.md` (commands, Kick Members, warning UX), then add a roadmap milestone. Until then, orchestrator should not dispatch work from this proposal.
