# Frequently Asked Questions

## Organizer setup

### How do I set up an organizer?

1. Open **System Console → Plugins → Community Admin**.
2. Use the ScopeEditor UI: search for a user, assign teams and optional channels.
3. Click **Save**.
4. The organizer can now open **Community Members** from any channel header in their assigned teams.

See the [configuration guide](configuration.md) for full details.

### Can I have multiple organizers?

Yes. The ScopeEditor supports an unlimited number of organizer entries. Each organizer has an independent scope (teams, channels, permissions, rate limits).

### How do I remove an organizer?

Delete the organizer entry in the ScopeEditor JSON or use the UI to remove them, then click **Save**. The user will no longer have access to the Community Members panel.

### What channels can my organizers see?

Organizers see only channels explicitly listed in their `channels` array, or all **public** channels in teams listed in their `all_channels_in_teams` array. They cannot see private channels unless those are explicitly added.

## Password reset

### What if the password reset doesn't work?

1. Confirm `ServiceSettings.EnableLocalMode: true` is set in the Mattermost server config.
2. Verify `/mattermost/bin/mmctl` exists inside the Mattermost container.
3. Check that the username passes validation (`^[a-z0-9._-]+$`).
4. If the reset fails silently, check the Mattermost server logs for `mmctl --local` errors.

See the [troubleshooting section in operations.md](operations.md#password-reset-fails).

### Can organizers reset their own password?

No. The plugin blocks self-resets server-side. A system administrator must reset an organizer's password through the Mattermost System Console.

### Are passwords stored in the audit log?

No. Passwords are generated server-side, handed to the organizer via an ephemeral banner, and are never written to the audit log or KV store.

## User management

### What happens when I remove a user from a team?

The user loses access to that team and all its channels. Their Mattermost account is **not** deactivated — they can still log in and access other teams where they are members.

### Can organizers create users in any team?

No. Organizers can only create users in teams where they are assigned as an organizer (defined in their `teams` array in ScopeConfig).

### Why are no channels showing in the Create User dropdown?

The channel dropdown loads from `GET /me`. If the organizer's scope uses `all_channels_in_teams`, that endpoint expands those team IDs into live public channel options. If no channels appear, verify:
- The organizer has at least one team with public channels assigned
- `all_channels_in_teams` contains the correct team IDs

## Technical

### What Mattermost versions are supported?

Mattermost **6.2.1+**, validated on **11.8.x Team / Entry Edition**.

### Why does the plugin use `mmctl --local` for password resets?

The Mattermost Plugin API does not expose an `UpdatePassword` method. The plugin uses a controlled `exec.Command` call to `mmctl --local user change-password` inside the Mattermost container as a secure workaround.

### Is this plugin on the Mattermost Marketplace?

Not yet. Install via release tarball from the [Releases page](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/releases).

### Do mobile clients support the plugin webapp?

No. Mobile clients do not load plugin webapp bundles. Organizers on mobile use the `/community-admin` slash commands instead.

### Why does the plugin ID start with `com.lalbers.`?

The plugin ID `com.lalbers.community-admin` is the install/upgrade key in Mattermost's plugin registry. Changing it would orphan existing installations, so it remains stable even though the repository moved to `github.com/lucas-albers-lz4`.
