# E2E tests

Full documentation: [../docs/testing.md](../docs/testing.md)

## Quick run

```sh
npm install
cp .env.example .env    # set ORGANIZER_PASSWORD, NON_ORGANIZER_PASSWORD
npm test                # Playwright (6 specs; excludes screenshot capture)
bash scripts/api-smoke.sh
```
