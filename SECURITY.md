# Security Policy

Audit ledger (what was checked, proven, fixed, still open): [docs/security-review.md](docs/security-review.md). Start there for the next review.

## Plugin privilege model

`com.lalbers.community-admin` runs with Mattermost server-level plugin API privileges.
All mutating operations pass through the `authz` package before calling Plugin API methods.

## Password reset bridge

Admin password reset uses controlled `mmctl --local` execution inside the Mattermost
container (see [docs/design.md](docs/design.md)):

- Fixed binary path: `/mattermost/bin/mmctl`
- Username validated before exec
- Password generated server-side only; process argv receives a bcrypt hash with `--hashed`
  (plaintext never appears in `/proc/*/cmdline`)
- 30 second timeout
- Passwords are never written to audit logs or KV store
- Audit `client_ip` uses Mattermost plugin context `IPAddress` when present
  (falls back to `RemoteAddr`; forwarded headers are not parsed)
- Internal/mmctl errors include mmctl output server-side and are not returned to organizers
  (5xx responses are redacted to a generic internal error)

## Reporting issues

Report security issues privately to the repository owner. Do not file public issues for
undisclosed vulnerabilities.

## Operator responsibilities

- Configure organizers by **user ID** in System Console
- Revoke access by saving **valid** JSON that omits the organizer (invalid JSON keeps the last-known-good allowlist until restart; see the ledger)
- Review audit log via `GET /plugins/com.lalbers.community-admin/api/v1/audit` (system admin)
- Keep break-glass admin scripts available for your deployment

## Webapp test hook

The webapp exposes `window.__communityAdminOpenPanel` for automated e2e tests. It only opens the RHS shell; `PanelWrapper` still calls `GET /me` and renders nothing for non-organizers.
