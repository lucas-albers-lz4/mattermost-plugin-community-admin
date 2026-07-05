# User guide (organizers)

## Opening the Community Members panel

On desktop/web, organizers open the right-hand sidebar panel:

1. Log in to Mattermost.
2. Open any channel in a team where you are configured as an organizer.
3. Click the **Community Members** button in the **channel header** (users icon).

![Where to open the Community Members panel](../images/community-admin/01-channel-header.png)

*Click the users icon in the channel header, or find **Community Members** in the product menu (grid icon, top left) on some Mattermost editions.*

On Mattermost 11 Entry Edition the item may also appear under the product switch menu (grid icon, top left) depending on server skin; if not visible, use the channel header button.

> The panel performs an organizer check (`GET /me`). Non-organizers see nothing even if they trigger the open action.

## Panel actions

| Action | Steps | Result |
|--------|-------|--------|
| List / search | Open panel; use search box | Shows users in your scoped teams/channels |
| Create user | **Create user** → fill username, names, team → **Create** | Credential banner with site URL, username, password |
| Reset password | **Reset password** on a user row | New credential banner; old password stops working |
| Remove from team | **Remove from {team}** on a user row | User loses access to that team |

### List and search

![Community Members panel showing the member list](../images/community-admin/02-panel-list.png)

*The panel lists members in your assigned teams. Use the search box to find someone by username.*

![Search box filtering the member list](../images/community-admin/03-search-filter.png)

*Type part of a username and click **Search** to narrow the list.*

### Create a new member

![Create user form](../images/community-admin/04-create-form.png)

*Click **Create user**, fill in username and name, pick a team, then click **Create**.*

![Credential handoff banner after creating a user](../images/community-admin/05-credentials-create.png)

*Copy the credential text and send it to the new member privately (text, email, or in person). Passwords are not stored in the audit log.*

### Reset password or remove from team

![Reset password and remove-from-team buttons on a member row](../images/community-admin/06-row-actions.png)

*Use **Reset password** when someone forgets their login. Use **Remove from {team}** when they should no longer access that team.*

![Credential banner after a password reset](../images/community-admin/07-credentials-reset.png)

*Share the new password the same way as when you create an account.*

Copy the credential banner text and send it to the new member out-of-band. Passwords are not stored in the audit log.

## Slash commands (mobile fallback)

When the web panel is hard to use on a phone, type a slash command in any channel where you have permission to post:

```
/community-admin reset-password USERNAME
/community-admin remove-from-team USERNAME TEAM_NAME
```

![Slash command typed in the message box](../images/community-admin/08-slash-command.png)

*Type the command in the message box at the bottom of a channel. The response is visible only to you.*

Responses are ephemeral (visible only to you). Example:

```
/community-admin reset-password testuser.alpha
```

## What organizers cannot do

- Reset passwords for themselves or system administrators (protected targets).
- Manage users outside assigned teams/channels.
- Create usernames with invalid characters (validation error).
