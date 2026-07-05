# Configuration

## System Console

**System Console → Plugins → Community Admin → Organizer scope configuration**

The custom ScopeEditor UI supports:

1. **Site URL** — included in credential handoff text (e.g. `https://chat.example.org`).
2. **Organizers** — add by username (resolved to user ID), assign teams and optional channels.
3. **Raw JSON** — advanced edit; Save enables when JSON is dirty.

After **Resolve and add**, click **Save** to persist. Reload the page to confirm values stick.

## ScopeConfig JSON shape

```json
{
  "site_url": "https://example.com",
  "organizers": [
    {
      "user_id": "<mattermost-user-id>",
      "display_username": "organizer.name",
      "teams": ["<team-id>"],
      "channels": ["<team-id>:<channel-id>"],
      "all_channels_in_teams": false,
      "allow_global_deactivate": false
    }
  ]
}
```

- **user_id** is authoritative; usernames are display-only.
- **teams** limits which team memberships an organizer can manage.
- **channels** (optional) further restricts visiblity when not using `all_channels_in_teams`.
- Only system administrators can edit this setting.

## Server prerequisites

| Setting | Required | Why |
|---------|----------|-----|
| `PluginSettings.Enable` | yes | Plugins must be enabled |
| `ServiceSettings.EnableLocalMode` | yes | Password reset via `mmctl --local` |
| `ServiceSettings.EnableOnboardingFlow` | optional | Disable on test to reduce UI overlays during e2e |

## Test instance example

Operators running a private test stack may use a dedicated hostname and team (see [testing.md](testing.md)). Production configuration should use your real Site URL and team IDs.
