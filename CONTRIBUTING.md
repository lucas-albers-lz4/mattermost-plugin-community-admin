# Contributing

Thank you for your interest in improving Community Admin.

## Development setup

```sh
git clone https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin.git
cd mattermost-plugin-community-admin
make install-dev-tools   # golangci-lint, govulncheck; brew install gitleaks (or apt on Linux)
make setup-hooks         # install pre-commit hook
go test ./...
cd webapp && npm install && npm run build && cd ..
make dist
```

### Pre-commit hook

After `make setup-hooks`, each commit runs:

1. **gitleaks** — secret scan on staged files
2. **go vet** — compile-time checks
3. **golangci-lint** — format (gofmt/gofumpt/goimports), lint, **gosec**
4. **go test** — unit tests
5. **govulncheck** — dependency vulnerability scan (if installed)

Skip once in an emergency: `git commit --no-verify` (use sparingly).

## CI notes

GitHub Actions uses Mattermost’s reusable `plugin-ci` workflow. Log lines about **Node 20 deprecation** or **`punycode` DEP0040** come from upstream Action runtimes (`setup-go`, artifact upload/cache cleanup), not from this plugin’s app code. They are safe to ignore while jobs stay green.

- App/CI Node for the webapp is controlled by [`.nvmrc`](.nvmrc) (Node 24.x).
- Do **not** set `ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION=true` to quiet the warning — that opts back into Node 20.

Warnings should disappear when Mattermost bumps those Action pins to runtimes that declare Node 24.

## Pull requests

1. Open an issue for significant changes when possible.
2. Keep changes focused; match existing Go and TypeScript style.
3. Ensure pre-commit passes (`make setup-hooks` then commit).
4. Update docs when behavior or configuration changes.

## Security

Do not open public issues for undisclosed vulnerabilities. See [SECURITY.md](SECURITY.md).

The audit ledger is [docs/security-review.md](docs/security-review.md). PRs that change authorization, the mmctl password bridge, audit/rate-limit KV, ScopeConfig, or GitHub workflows must update that file in the same PR (coverage-map date, control row, open/resolved findings). Do not re-file items listed as accepted residuals.
