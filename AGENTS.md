# Agent notes — mattermost-plugin-community-admin

Plugin ID: `com.lalbers.community-admin`. Module path: `github.com/lucas-albers-lz4/mattermost-plugin-community-admin`.

## Release

Source of truth: [docs/testing.md](docs/testing.md) **Pre-release checklist**.

Before a release/tag:

1. `go test ./server/...` then `make dist`
2. Install on the CIDR-gated test instance
3. Run `e2e/scripts/api-smoke.sh` and `cd e2e && npm test`

Do **not** add nightly e2e CI — ship volume is low; smoke/Playwright are release-time only. Never commit `e2e/.env` or `dist/`.

## Security reviews

[docs/security-review.md](docs/security-review.md) is the **single source of truth** for review state: coverage map, controls with proof class (`host` | `lab` | `manual`), open findings, accepted residuals, and the review procedure. Start there. Do not re-derive scope or reopen residuals without new evidence.

Feature PRs that touch authz, mmctl/password, audit KV, ScopeConfig, or GitHub workflows must update the ledger in the **same PR**. Stubbed mmctl / in-memory KV tests are not proof of the live path (false-green rule). Cursor rule: `.cursor/rules/security-audit.mdc`.

## Defaults

- Minimal diffs; do not commit or push unless asked
- Longer guides live under `docs/` — do not duplicate them in README
- Cursor rules: `.cursor/rules/project.mdc`, `.cursor/rules/security-audit.mdc`
