# Configuration

## System Console

**System Console → Plugins → Community Admin → Organizer scope configuration**

The custom ScopeEditor UI supports:

1. **Site URL** — included in credential handoff text (e.g. `https://chat.example.org`).
2. **Organizers** — pick a user from a searchable dropdown, multi-select teams, and optionally multi-select channels (populated from the live server). Leave channels empty to allow all public channels in the selected teams.
3. **Raw JSON** — advanced edit; Save enables when JSON is dirty.

After **Add organizer**, click **Save** to persist. Reload the page to confirm values stick.

Dropdowns call system-admin-only plugin APIs (`GET /api/v1/admin/users`, `/admin/teams`, `/admin/teams/{id}/channels`). Non-admins cannot use them.

## ScopeConfig JSON shape

```json
{
  "version": 1,
  "site_url": "https://example.com",
  "email_domain": "community.local",
  "organizers": [
    {
      "user_id": "<mattermost-user-id>",
      "display_username": "organizer.name",
      "teams": [
        {"id": "<team-id>", "name": "Team Display Name"}
      ],
      "channels": [
        {"id": "<channel-id>", "team_id": "<team-id>", "name": "Town Square"}
      ],
      "all_channels_in_teams": ["<team-id>"],
      "permissions": {
        "create_user": true,
        "edit_profile": true,
        "reset_password": true,
        "manage_membership": true,
        "remove_from_team": true,
        "deactivate_globally": false
      },
      "rate_limits": {
        "creates_per_hour": 20,
        "password_resets_per_hour": 10
      }
    }
  ]
}
```

- **user_id** is authoritative; usernames are display-only.
- **teams** is an array of `{id, name}` objects (not bare ID strings). At least one team is required for Create User dropdowns.
- **channels** is optional explicit `{id, team_id, name}` allowlist.
- **all_channels_in_teams** is a list of team IDs whose **public** channels are allowed (and listed in Create User). Prefer this when you want every public channel in the team without listing each one. Use `[]` when restricting to explicit `channels` only.
- Only system administrators can edit this setting.

Create User loads teams/channels from `GET /me`. That endpoint expands `all_channels_in_teams` into live public channel options so the channel dropdown is not empty when `channels` is `[]`.

## Server prerequisites

| Setting | Required | Why |
|---------|----------|-----|
| `PluginSettings.Enable` | yes | Plugins must be enabled |
| `ServiceSettings.EnableLocalMode` | yes | Password reset via `mmctl --local` |
| `ServiceSettings.EnableOnboardingFlow` | optional | Disable on test to reduce UI overlays during e2e |

## Test instance example

Operators running a private test stack may use a dedicated hostname and team (see [testing.md](testing.md)). Production configuration should use your real Site URL and team IDs.
