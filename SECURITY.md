# Security Policy

## Plugin privilege model

`com.lalbers.community-admin` runs with Mattermost server-level plugin API privileges.
All mutating operations pass through the `authz` package before calling Plugin API methods.

## Password reset bridge

Admin password reset uses controlled `mmctl --local` execution inside the Mattermost
container (see [docs/design.md](docs/design.md)):

- Fixed binary path: `/mattermost/bin/mmctl`
- Username validated before exec
- Password generated server-side only
- 30 second timeout
- Passwords are never written to audit logs or KV store

## Reporting issues

Report security issues privately to the repository owner. Do not file public issues for
undisclosed vulnerabilities.

## Operator responsibilities

- Configure organizers by **user ID** in System Console
- Revoke access by removing organizer entries
- Review audit log via `GET /plugins/com.lalbers.community-admin/api/v1/audit` (system admin)
- Keep break-glass admin scripts available for your deployment

## Webapp test hook

The webapp exposes `window.__communityAdminOpenPanel` for automated e2e tests. It only opens the RHS shell; `PanelWrapper` still calls `GET /me` and renders nothing for non-organizers.
