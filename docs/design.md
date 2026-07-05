# Design notes

Target platform: Mattermost 11.x Team / Entry Edition (validated on 11.8.2).

## API validation summary

| Capability | Plugin API | Decision |
|------------|------------|----------|
| CreateUser with password | `CreateUser(&model.User{Password: ...})` | **Use** — set `EmailVerified: true`, `DisableWelcomeEmail: true` |
| UpdateUserActive | `UpdateUserActive(userID, active)` | **Use** for global deactivate/reactivate |
| Team/channel membership | `CreateTeamMember`, `DeleteTeamMember`, channel member APIs | **Use** |
| UpdateUser profile | `UpdateUser` for first/last name | **Use** |
| NotifyProps (push defaults) | `UpdateUser` with `NotifyProps["push"]="all"` | **Use** — avoids direct SQL |
| Admin password reset | No `UpdatePassword` in Plugin API | **Use mmctl --local fallback** |
| Auth boundary | `Mattermost-User-Id` header set by server on authenticated requests | **Reject empty header** |
| Non-organizer access | Custom allowlist by `user_id` | **403 via authz checker** |

## Password reset bridge

`mattermost-oci-deploy` enables `EnableLocalMode: true` on the Mattermost container. The plugin runs:

```
/mattermost/bin/mmctl --local user change-password <username> --password <generated>
```

Constraints:

- Fixed binary path only (`/mattermost/bin/mmctl`)
- Username validated with `^[a-z0-9._-]+$` before exec
- Password generated server-side (never from client)
- 30s subprocess timeout
- No shell interpolation (direct `exec.Command` args)
- Passwords are never written to audit logs or KV store

## Authorization model

- Organizers are configured by **user ID** in `ScopeConfig` (System Console).
- Each organizer has scoped `teams`, `channels`, and optional `all_channels_in_teams`.
- Protected targets (self-reset, system admins) are rejected server-side.
- Rate limiting applies to user creation (configurable threshold).

## Web UI vs mobile

- Desktop/web: RHS **Community Members** panel (plugin webapp).
- Mattermost 11 Entry Edition may not render `registerMainMenuAction` items in the product switch menu even when registered; the channel header button and RHS hook are the reliable entry points on test.
- Mobile clients do not load plugin webapp bundles. Organizers use `/community-admin` slash commands instead.

## Plugin HTTP API

Base path: `/plugins/com.lalbers.community-admin/api/v1`

| Method | Path | Description |
|--------|------|-------------|
| GET | `/me` | Organizer context (teams, permissions) |
| GET | `/users?q=` | List/search scoped users |
| POST | `/users` | Create user |
| POST | `/users/{id}/reset-password` | Reset password |
| DELETE | `/users/{id}/teams/{teamId}` | Remove from team |
| POST | `/resolve-scope` | Resolve usernames / team:channel (admin console) |
| GET | `/audit` | Audit log (system admin) |

See server handlers in `server/api.go` for the full surface.
