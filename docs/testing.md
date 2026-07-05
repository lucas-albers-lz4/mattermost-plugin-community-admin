# Testing

## Test environment

| Item | Value |
|------|-------|
| URL | `https://doomzilla.duckdns.org` |
| Instance | `mattermost-test` on deploy VM |
| Start | `ssh ubuntu@129.146.67.228 '/opt/mattermost/ops/manage-test-instance.sh start'` |
| Stop | `... manage-test-instance.sh stop` |

Runs must originate from an IP in `TEST_ALLOWED_CIDR` (your desktop by default).

## Manual exploratory checklist

Use this when validating UI behavior beyond automation.

| # | Area | Steps | Expected |
|---|------|-------|----------|
| A1 | Access gate | Log in as non-organizer; look for Community Members | No panel / no menu item |
| A2 | List | Open panel; search `alpha` | Scoped users listed; search filters |
| A3 | Create | Create `testuser.<name>` on team `test` | Credential banner; login works |
| A4 | Reset | Reset `testuser.alpha` | New password in banner |
| A5 | Remove | Remove user from team | User loses team access |
| A6 | Slash | `/community-admin reset-password testuser.beta` | Ephemeral credential text |
| A7 | Guardrails | Reset self; invalid username `Bad User!` | Errors (protected / validation) |
| A8 | Admin console | Resolve organizer; edit Site URL; raw JSON | Save persists after reload |

Organizer accounts on test: `lucasalbers`, `test.organizer`. Sample members: `testuser.alpha`, `testuser.beta`.

## API smoke (no browser)

```sh
cd e2e
cp .env.example .env   # fill ORGANIZER_PASSWORD, NON_ORGANIZER_PASSWORD
bash scripts/api-smoke.sh
```

Covers A1–A5 and A7; attempts A6 slash command via `/api/v4/commands/execute`. Automatically resets `testuser.beta` password if login fails.

## Playwright e2e

```sh
cd e2e
npm install
cp .env.example .env
npm test                 # headless (uses system Chrome via channel: 'chrome')
npm run test:ui          # interactive debugger
npm run test:headed      # visible browser
```

### Specs (`tests/organizer-panel.spec.ts`)

| Test | Flow |
|------|------|
| `organizer_can_open_panel` | Login → open panel → no error |
| `organizer_lists_scoped_users` | `testuser.alpha` / `testuser.beta` visible |
| `organizer_creates_user` | Create `testuser.pw.<timestamp>` |
| `organizer_resets_password` | Reset `testuser.alpha` |
| `organizer_removes_from_team` | Create then remove user |
| `non_organizer_no_panel` | No menu item; panel stays hidden |

The auth fixture opens the panel via channel header button when visible, otherwise via `window.__communityAdminOpenPanel` (test hook registered by the webapp).

### Documentation screenshots

Capture PNGs for the [user guide](user-guide.md) and deploy-repo parent docs:

```sh
cd e2e
npm run screenshots              # headless
npm run screenshots:headed       # visible browser for debugging
```

Outputs to `docs/images/community-admin/`. Credential passwords are redacted before capture. See [docs/images/community-admin/README.md](images/community-admin/README.md) for the file manifest.

Sync a subset to the deploy repo:

```sh
/path/to/mattermost-oci-deploy/scripts/sync-community-admin-screenshots.sh
```

Screenshot specs are excluded from `npm test` (see `playwright.config.ts` `testIgnore`).

### CI note

GitHub-hosted runners cannot reach the IP-restricted test host without a tunnel or self-hosted runner in the allowed CIDR.

### Dev helper

`e2e/scripts/debug-menu.mjs` — logs plugin load and menu contents for UI troubleshooting.
