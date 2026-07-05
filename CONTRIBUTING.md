# Contributing

Thank you for your interest in improving Community Admin.

## Development setup

```sh
git clone https://github.com/lalbers/mattermost-plugin-community-admin.git
cd mattermost-plugin-community-admin
make install-dev-tools   # golangci-lint, govulncheck; brew install gitleaks
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

## Pull requests

1. Open an issue for significant changes when possible.
2. Keep changes focused; match existing Go and TypeScript style.
3. Ensure pre-commit passes (`make setup-hooks` then commit).
4. Update docs when behavior or configuration changes.

## Security

Do not open public issues for undisclosed vulnerabilities. See [SECURITY.md](SECURITY.md).
