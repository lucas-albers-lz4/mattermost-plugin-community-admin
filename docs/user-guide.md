# User guide (organizers)

## Opening the Community Members panel

On desktop/web, organizers open the right-hand sidebar panel:

1. Log in to Mattermost.
2. Open any channel in a team where you are configured as an organizer.
3. Click the **Community Members** button in the **channel header** (users icon).

On Mattermost 11 Entry Edition the item may also appear under the product switch menu (grid icon, top left) depending on server skin; if not visible, use the channel header button.

> The panel performs an organizer check (`GET /me`). Non-organizers see nothing even if they trigger the open action.

## Panel actions

| Action | Steps | Result |
|--------|-------|--------|
| List / search | Open panel; use search box | Shows users in your scoped teams/channels |
| Create user | **Create user** → fill username, names, team → **Create** | Credential banner with site URL, username, password |
| Reset password | **Reset password** on a user row | New credential banner; old password stops working |
| Remove from team | **Remove from {team}** on a user row | User loses access to that team |

Copy the credential banner text and send it to the new member out-of-band. Passwords are not stored in the audit log.

## Slash commands (mobile fallback)

In any channel where you have permission to post:

```
/community-admin reset-password USERNAME
/community-admin remove-from-team USERNAME TEAM_NAME
```

Responses are ephemeral (visible only to you). Example:

```
/community-admin reset-password testuser.alpha
```

## What organizers cannot do

- Reset passwords for themselves or system administrators (protected targets).
- Manage users outside assigned teams/channels.
- Create usernames with invalid characters (validation error).
