# Agent notes — mattermost-plugin-community-admin

Plugin ID: `com.lalbers.community-admin`. Module path: `github.com/lucas-albers-lz4/mattermost-plugin-community-admin`.

## Release

Source of truth: [docs/testing.md](docs/testing.md) **Pre-release checklist**.

Before a release/tag:

1. `go test ./server/...` then `make dist`
2. Install on the CIDR-gated test instance
3. Run `e2e/scripts/api-smoke.sh` and `cd e2e && npm test`

Do **not** add nightly e2e CI — ship volume is low; smoke/Playwright are release-time only. Never commit `e2e/.env` or `dist/`.

## Defaults

- Minimal diffs; do not commit or push unless asked
- Longer guides live under `docs/` — do not duplicate them in README
- Cursor project rule: `.cursor/rules/project.mdc`
