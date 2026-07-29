# Changelog

All notable changes to this project are documented here. Versioning follows [Semantic Versioning](https://semver.org/).

## [1.0.2] - 2026-07-29

Security and reliability hardening after the 1.0.1 dependency release.

### Fixed

- Authz: harden scope, roles, and channel wildcard policy
- Config: default empty `rate_limits`; cache ScopeConfig and keep last-known-good on parse fail
- Audit: atomic KV updates for rate limits and audit index; TargetID on batch; prune expired index
- Password bridge: avoid plaintext on mmctl argv; sanitize API errors; harden RNG
- Slash commands: audit and rate-limit; fail OnActivate if command registration fails
- Batch import: rate-limit imports, require team, skip unsafe existing-user enroll; clearer validation errors
- API/UI: limit JSON bodies; ScopeEditor safety; list pagination and remove-team targeting hygiene

### Changed

- deps: bump mattermost `server/public` to v0.4.3

### Tests

- Add UserService and MembershipService unit tests
- Require e2e `TEST_URL`; polish API/UX nits

## [1.0.1] - 2026-07-28

Security/cleanup dependency bumps (Dependabot npm/Go, npm overrides) and docs polish.

## [1.0.0] - 2026-07-05

Initial public release (MVP).

### Added

- Organizer allowlist with team/channel scope (System Console ScopeEditor + JSON)
- RHS **Community Members** panel: list, search, create user, reset password, remove from team
- Server API under `/plugins/com.lalbers.community-admin/api/v1`
- `/community-admin` slash commands for mobile (`reset-password`, `remove-from-team`)
- Password reset via controlled `mmctl --local` bridge
- Audit log endpoint for system administrators
- Authz layer with protected targets and create-rate limiting
- Playwright e2e suite and API smoke script

### Requirements

- Mattermost 6.2.1+ (validated on 11.8.x Team / Entry Edition)
- `ServiceSettings.EnableLocalMode: true` for password reset

[1.0.2]: https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/releases/tag/v1.0.2
[1.0.1]: https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/releases/tag/v1.0.1
[1.0.0]: https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/releases/tag/v1.0.0
