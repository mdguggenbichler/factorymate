# Users & registration

FactoryMate supports two onboarding paths. **Discord `/register` is primary**; web invite links are break-glass recovery only.

## Discord registration (recommended)

1. Admin configures role mappings under **Settings → Discord** so the right members can self-register.
2. Player runs **`/register`** in Discord.
3. Player fills in in-game username and dashboard password in the modal.
4. FactoryMate creates the account, links Discord identity, and saves the pending in-game name.
5. If **auto-approve** is on (default), the player can log in immediately.
6. If auto-approve is off, an admin approves via Discord buttons, admin commands, or the web pending-approvals queue.

### Admin-initiated registration

Admins can run **`/register user:@someone`** to invite a specific person. The bot DMs them a button to complete registration. These registrations are always auto-approved.

### Linking an existing account

If someone already has a web-only account, they run **`/link`** in Discord to attach their Discord identity.

## Web invite links (break-glass)

**Settings → Users → Advanced**

Admins can create single-use invite links (7-day expiry) for recovery when Discord registration is unavailable.

1. Create invite → copy URL.
2. User visits `/invite/:token` → sets username and password.
3. User should **`/link`** Discord afterward if possible.

Do not use web invites as the normal onboarding path.

## User management (admin)

**Settings → Users**

| Panel | Purpose |
| --- | --- |
| User table | All accounts: username, role, Discord link, player mapping, status |
| Pending approvals | Registrations waiting for admin action (when auto-approve is off) |
| Unmapped players | In-game names seen on the server without a linked FM user |
| Advanced invites | Break-glass web invite creation |

### Roles

| Role | Access |
| --- | --- |
| `admin` | Full dashboard and settings |
| `viewer` | Read-only dashboard, personal Discord commands |

Change roles in the user table. Discord role mappings apply at registration time; use **`/sync-roles`** to re-apply mappings.

### Player mapping

Each user can have an associated in-game player name:

- Set at `/register` (saved as pending until the player appears on the server)
- Updated by the user via **`/set-player`**
- Overridden by admins in **Settings → Users**

When a matching player joins the FRM player list, FactoryMate auto-links `player_id`.

### Unlink Discord

Admins can unlink a user's Discord identity from the user table or via **`/unlink @user`**. The FactoryMate account remains; the user can **`/link`** again.

## Registration approval

When **auto-approve** is off:

1. New `/register` submissions get `pending_approval` status.
2. Pending users cannot log in or run player commands.
3. Admins approve or reject via:
   - Discord DM buttons on the approval notification
   - **`/registrations approve`** / **`/registrations reject`**
   - **Settings → Users** pending queue

Rejected registrations are removed after audit logging.

## Password reset

Admins trigger **`/password-reset @user`** in Discord. The user receives a DM with reset instructions.

## Tips for small groups

- Keep **auto-approve on** and control access via Discord role gating (who can `/register`).
- Turn auto-approve **off** if the guild is semi-public or join credentials are sensitive.
- Set **`FACTORYMATE_PUBLIC_URL`** so welcome DMs and `/help` show the correct dashboard link.
