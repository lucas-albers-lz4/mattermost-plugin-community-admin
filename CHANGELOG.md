# Changelog

All notable changes to this project are documented here. Versioning follows [Semantic Versioning](https://semver.org/).

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

[1.0.0]: https://github.com/lalbers/mattermost-plugin-community-admin/releases/tag/v1.0.0
