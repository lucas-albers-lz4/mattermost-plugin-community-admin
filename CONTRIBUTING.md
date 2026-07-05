# Contributing

Thank you for your interest in improving Community Admin.

## Development setup

```sh
git clone https://github.com/lalbers/mattermost-plugin-community-admin.git
cd mattermost-plugin-community-admin
go test ./...
cd webapp && npm install && npm run build && cd ..
make dist
```

Deploy to a local or test Mattermost with `make deploy` (requires `MM_SERVICESETTINGS_SITEURL` and admin credentials) or upload `dist/*.tar.gz` via System Console.

## Pull requests

1. Open an issue for significant changes when possible.
2. Keep changes focused; match existing Go and TypeScript style.
3. Run `go test ./...` before submitting.
4. Update docs when behavior or configuration changes.

## Security

Do not open public issues for undisclosed vulnerabilities. See [SECURITY.md](SECURITY.md).
