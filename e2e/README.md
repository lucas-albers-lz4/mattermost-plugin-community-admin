# E2E tests

Playwright and API smoke tests for the Community Admin plugin.

**Full documentation:** [../docs/testing.md](../docs/testing.md)

## Quick run

```sh
npm install
cp .env.example .env    # set ORGANIZER_PASSWORD, NON_ORGANIZER_PASSWORD
npm test                # Playwright (6 specs)
bash scripts/api-smoke.sh
```

Uses system Chrome (`channel: 'chrome'` in `playwright.config.ts`). Optional: `npx playwright install chromium` if you prefer bundled Chromium.

## Files

| Path | Purpose |
|------|---------|
| `tests/organizer-panel.spec.ts` | Core RHS panel flows |
| `fixtures/auth.ts` | Login, onboarding dismiss, panel open |
| `scripts/api-smoke.sh` | Browserless checklist (A1–A7, A6) |
| `scripts/debug-menu.mjs` | Menu/plugin load diagnostics |
| `.env.example` | `TEST_URL`, organizer credentials |
