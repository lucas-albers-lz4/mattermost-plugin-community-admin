# Security audit ledger — mattermost-plugin-community-admin

Record of what was **checked**, **proven**, **fixed**, **accepted**, and **still open**.
Start here for future security reviews so prior work is not re-done blindly.

Operator guidance: [SECURITY.md](../SECURITY.md). Design: [design.md](design.md).
Pattern: [usrmanage `docs/security-review.md`](https://github.com/lucas-albers-lz4/usrmanage/blob/main/docs/security-review.md) + [regexproof `docs/SECURITY-AUDIT.md`](https://github.com/lucas-albers-lz4/regexproof/blob/main/docs/SECURITY-AUDIT.md).

## Three goals

| Goal | Meaning |
|------|---------|
| **Exploit** | Authz bypass, mmctl argv/shell injection, password leakage, XSS, IDOR / scope escape |
| **Integrity** | Audit loss/orphan, rate-limit escape, config revoke fail-open, partial create leftover membership |
| **Supply chain** | CI reusable workflow, release TCB, Dependabot, unsigned checksums, proof gates that cannot fail |

## Surface coverage map

Update the date and findings columns in the same PR as the review. Oldest date is where the next pass should start.

| Surface | Where | Last reviewed | Open findings |
|---------|-------|---------------|---------------|
| Authz checker | `server/authz/` | 2026-08-12 (S1 remediation) | none new |
| HTTP API + errors | `server/api.go`, `api_handlers.go` | 2026-08-12 | none new |
| Password / mmctl | `server/service/users.go`, `password.go` | 2026-08-12 | none new |
| Audit + rate-limit KV | `server/service/audit.go` | 2026-08-12 | I3 |
| Batch import | `server/service/batch.go` | 2026-08-12 | none new (I5 deferred #36) |
| Slash commands | `server/command/` | 2026-08-12 | none (parity with HTTP holds) |
| ScopeConfig parse/cache | `server/configuration.go`, `config/schema.go` | 2026-08-12 | I1 documented residual |
| Webapp RHS / ScopeEditor | `webapp/src/` | 2026-08-12 (Opus XSS sweep) | W1 (CSRF header only) |
| CI / release | `.github/workflows/` | 2026-08-13 (R1/C2 remediation) | R3, C1 |
| Z3 username proof | `proof_username_whitelist.py` | 2026-08-13 (P1 remediation) | V2 |
| e2e / smoke | `e2e/` | 2026-08-12 | none new |

## How to re-verify

| Command | What it covers | Class |
|---------|----------------|-------|
| `go test ./server/...` | Authz matrix, IsSystemAdmin token+fuzz, rate-limit CAS logic, audit prune, batch skip-existing, JSON parse last-known-good | host |
| `python3 proof_username_whitelist.py` | Z3 alphabet-disjointness vs shell metachar set (not argv semantics) | host |
| `e2e/scripts/api-smoke.sh` | Live Mattermost: non-organizer 403, organizer list/reset | lab |
| `cd e2e && npm test` | Playwright RHS flows | lab |
| `rg -n 'exec\\.Command' server/` | New subprocess sites must go through `defaultChangePassword` | sweep |
| `rg -n 'dangerouslySetInnerHTML\\|innerHTML' webapp/src` | XSS sinks (currently none) | sweep |
| `gh api repos/:owner/:repo/code-scanning/alerts` | Include dismissed; unpinned-tag is won't-fix | sweep |

**False-green rule:** a test that stubs `changePassword` or uses in-memory `memKV` proves *logic*, not the live mmctl/plugin-KV path. Do not list those as proof of a `lab`-class control.

## Controls in force

**Proof class:** `host` (Go test or Z3) · `lab` (api-smoke / Playwright) · `manual` (code review only).

### Exploit

| Attack surface | Guard | Class | Proof |
|----------------|-------|-------|-------|
| Unauthenticated plugin HTTP | `mattermostAuthorizationRequired` rejects empty `Mattermost-User-Id` (header set by server, not the client) | host | `TestAPIUnauthorized` |
| Non-organizer access | `ResolveOrganizer` + `requireOrganizer` on mutating routes | host | `TestAuthorizationMatrix` · `TestAPIRequireOrganizerDenied` · lab A1 |
| Account-wide ops on out-of-scope users | `authorizeAccountWideTarget` requires team intersection **and** `allTeamsSubset` | host | matrix: reset/edit/deactivate cross-team → `ErrForbidden` |
| Protected targets | self, bots, elevated `system_*` roles except `system_user` / `system_guest`, peer organizers, username `calls` | host | matrix + `TestIsSystemAdminTokenBoundary` / `TestIsProtectedSystemRoles` / fuzz (#61) |
| Private channels via wildcard | `HasChannel` wildcard only when `channelIsOpen`; `GetChannelScope` uses `ChannelTypeOpen` | host | `TestHasChannelWildcardRequiresOpen` · `TestGetChannelScopeOpenAndPrivate` |
| Channel ID existence oracle | missing channel → same 403 as out-of-scope | host | `handleAddChannelMember` |
| Username → shell metachar / argv flags | charset `^[a-z][a-z0-9._-]*$` at create, batch, **and** reset; `exec.CommandContext` argv (no shell) with `--` before username | host | `TestValidateUsername` · `TestChangePasswordArgsSeparateUsernameFromFlags` · Z3 `proof_username_whitelist.py` |
| Password on `/proc` cmdline | bcrypt `--hashed`; plaintext generated server-side; HTTP create does not accept client password | manual | `users.go:63-71` · `handleCreateUser` omits `Password` — live mmctl argv remains untested; argument shape is host-tested |
| Password in audit/KV | `AuditEntry` has no password field; `recordAudit` never copies `result.Password` | host | struct + `TestResetPasswordSuccess` (stub) |
| mmctl stderr to organizer | `writeError` redacts `status >= 500` to `internal error`; slash returns generic text | host | `api_handlers.go:46-49` and admin list handlers |
| XSS in RHS / ScopeEditor | React JSX; no `dangerouslySetInnerHTML` / `innerHTML` | manual | sweep 2026-08-12 |
| e2e panel hook | `__communityAdminOpenPanel` only shows RHS; `PanelWrapper` still `GET /me` | manual | `webapp/src/index.tsx:29-56` |
| CSRF | `X-Requested-With` + Mattermost cookie CSRF; empty `Mattermost-User-Id` → 401 | manual | Opus: not exploitable today; **W1** if `ExperimentalStrictCSRFEnforcement` |

### Integrity

| Failure scenario | Guard | Class | Proof |
|------------------|-------|-------|-------|
| Concurrent create/reset quota | `SetAtomicWithRetries` on `rate_<actor>_<action>_<hour>` | host | `TestCheckAndIncrementConcurrent` (memKV — logic only, not plugin KV) |
| HTTP + slash + batch share create quota | same action key `create_user` / `reset_password` | host | `api_handlers.go` + `command.go` |
| Audit index RMW races | CAS on `audit_index`; cap 10k; 90-day prune; List alloc capped 500 (#49) | host | `TestAuditRecordConcurrent` · `TestAuditRecordPrunesExpired` |
| Audit dropped on KV error | HTTP `recordAudit` logs `Record` errors while preserving the mutation response | manual | `api_handlers.go:88-90` |
| Audit entry vs index crash window | **none** — `Set` then CAS index | — | **I3 open** |
| Invalid ScopeConfig live reload | last-known-good parsed allowlist (#39); first-load / process restart → empty organizers | host | `TestApplyParsedScopeConfigKeepsPreviousOnFailure` — **I1 residual** |
| Batch existing-user enroll | skip; do not mutate membership | host | `TestImportSkipsExistingWithoutMembership` |
| Batch duplicate team names | reject rather than overwrite (#26) | host | `buildTeamNameMap` |
| JSON / batch body DoS | 64 KiB JSON; 1 MiB / 200 rows batch; list `per_page` max 200 | host | `api_handlers.go:59,559,621` |
| Partial create leftover membership | deactivate-only cleanup | — | **deferred #36 / I5** |

### Supply chain

| Failure scenario | Guard | Class | Proof |
|------------------|-------|-------|-------|
| PR dependency vulns | `dependency-review.yml` on pull_request | manual | workflow |
| Dependabot security updates | repo setting (version updates disabled, `open-pull-requests-limit: 0`) | manual | `.github/dependabot.yml` |
| `pull_request_target` | not used | manual | sweep 2026-08-12 |
| CI reusable workflow pin | SHA-pinned `plugin-ci.yml`; bumps require intentional SHA updates | manual | `.github/workflows/ci.yml` |
| Release job token vs `npm ci` | `actions/checkout@v5` sets `persist-credentials: false`; release install uses `npm ci --ignore-scripts` | manual | `.github/workflows/release.yml` review |
| False-green release | `go test ./server/...` runs before `make dist` in the release job | host | workflow step; tag publication is blocked when server tests fail |
| Release checksums | `SHA256SUMS` same-job, unsigned, no provenance | — | **R3 open** |
| Dev-scope npm CVEs | `webapp/package.json` overrides pin patched versions; lockfile refreshed and verified with `npm ls` | host | `cd webapp && npm ls brace-expansion nanoid` |
| Action major tags (`@v5`, `@v2`) | fleet won't-fix (CodeQL alert 3 dismissed) | manual | same as regexproof |
| Z3 proof in merge gate | `username-proof` CI job, `make proof-username`, and staged-change pre-commit hook when `z3` is importable | host | `python3 proof_username_whitelist.py` |
| Required status checks | **none** on `master` | — | **C1 open** |
| e2e secrets | `e2e/.env` gitignored; login body uses a restricted `mktemp` file and password is piped to curl stdin | manual | `e2e/scripts/api-smoke.sh`, `e2e/.env.example` |
| js-yaml CVE-2026-59870 | [PR #62](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/62) merged | manual | override `4.3.1`; lockfile has one `4.3.1` |

## Open findings

From the 2026-08-12 multi-model pass (Opus 5 supply-chain · Grok 4.6 exploit · GPT-5.6 Sol integrity · GLM 5.2 ledger). Parent re-checked High claims before recording.

| ID | Sev | Area | Notes |
|----|-----|------|-------|
| R3 | Medium | Release integrity | `SHA256SUMS` built in the same job as the tarball, unsigned, no provenance. `id-token: write` sits on CI (unused here) instead of release. Opus. |
| C1 | Medium | False-green | `master` has no required status checks and no required PR reviews. Opus (`GET /branches/master/protection`). |
| V2 | Low | Ledger honesty | Z3 proves alphabet disjointness only, not argv/`cobra` semantics or length. Do not cite as “username injection impossible.” |
| I3 | Low | Audit KV | Entry `Set` then index CAS; crash orphans an unlistable key. |
| I6 | Low | List amplification | `SearchInTeams` applies `per_page` per team. Organizer-trusted. |
| R5 | Low | Deploy hygiene | Documented client-bundle copy is never removed on plugin delete. Opus. |
| W1 | Low | Webapp CSRF | Only `X-Requested-With`, not `X-CSRF-Token`. Not exploitable today; breaks if strict CSRF is enabled. Opus. |

## Resolved findings

| Issue | Area | Resolved by |
|-------|------|-------------|
| V1 / [#63](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/63) | mmctl username argv injection | [PR #67](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/67) first-letter username constraint and `--` separator |
| V4 / [#63](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/63) | Admin list error redaction | This PR — list handlers now use `writeError` for 500s |
| V5 / [#63](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/63) | mmctl argv false-green | This PR — existing `TestChangePasswordArgsSeparateUsernameFromFlags` asserts the production argument shape; live mmctl remains a manual gap |
| I7 / [#63](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/63) | Admin pagination overflow | This PR — overflow-safe bounds helper and unit test |
| C4 / [#63](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/63) | Smoke protected-target false-green | This PR — A7 now fails on any status other than 403 |
| E1–E3 / [#63](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/63) | e2e secret hygiene | This PR — mktemp cleanup, stdin login body, and scrubbed example hostname |
| S1 / [#63](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/63) | Protected elevated `system_*` roles | [PR #68](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/68) |
| I2 / [#63](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/63) | HTTP audit failures are logged | [PR #69](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/69) |
| [#60](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/60) | Authz `IsSystemAdmin` substring | [PR #61](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/61) exact token match |
| [#37](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/37) | JSON body limits / channel authz order | [PR #43](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/43) |
| [#38](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/38) | Audit TargetID / actor / prune | [PR #44](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/44) |
| [#39](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/39) | Config cache / empty fallback | [PR #45](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/45) (last-known-good; see I1 residual) |
| [#17](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/17) | KV RMW races | [PR #31](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/31) CAS |
| [#18](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/18) | `allTeamsSubset` on account-wide ops | PR #30 stack |
| [#19](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/19) | Slash audit / rate-limit | [PR #33](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/33) |
| [#20](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/20) | Plaintext mmctl argv | [PR #32](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/32) `--hashed` |
| [#23](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/23) | Batch unlimited creates | [PR #34](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/34) |
| [#25](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/25) | Wildcard auto-grant | PR #30 / #35 |
| [#26](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/26) | Duplicate team-name map | batch `buildTeamNameMap` |
| [#28](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/28) | `IsSystemAdmin` Contains | superseded by #60/#61 |
| [#29](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/29) | CreateUser first team only | PR #30 |
| [#49](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/49) | Audit List alloc | [PR #49](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/49) |
| [#56](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/56) | Dependabot fast-uri / postcss | [PR #55](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/55) |
| Dependabot 117 / js-yaml CVE-2026-59870 | Webapp override | [PR #62](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/62) |
| R4 | Dev-scope npm CVEs | [PR #65](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/65) — `brace-expansion` 1.1.18 / 2.1.4 and `nanoid` 3.3.17 overrides with refreshed lockfile |
| R2 | Release TCB credentials and lifecycle scripts | [PR #66](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/66) — checkout credentials disabled; npm lifecycle scripts disabled |
| C3 | False-green release | [PR #66](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/66) — server tests run before `make dist` |
| R1 | CI TCB | This PR — reusable `plugin-ci` workflow pinned to a verified commit SHA |
| C2 | False-green | This PR — removed the upstream-incompatible no-op nightly schedule |
| P1 | Proof gate | This PR — wired the Z3 proof into CI, Makefile, and staged-change pre-commit |
| Z3 username alphabet | Injection alphabet | [PR #59](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/59) (scope: chars only) |

## Accepted residuals

Do not re-open without new evidence.

| Accepted risk | Rationale |
|---------------|-----------|
| `EnableLocalMode` / mmctl local socket | Anyone who can exec in the Mattermost container can already administer it. Operator requirement. |
| Last-known-good ScopeConfig on live parse fail (I1) | Intentional after #24/#39 (invalid JSON must not wipe all organizers). Restart with still-invalid JSON fail-closes to empty. Revoke by saving **valid** JSON that omits the user. |
| Authz check-then-act TOCTOU (I4) | Promotion to `system_admin` / organizer during reset needs a concurrent system admin. Same class as usrmanage session-login I3. |
| Partial create membership rollback (I5) | Explicitly deferred in [#36](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/36). Failed create deactivates the user; leftover memberships stay until cleanup. |
| Password in organizer HTTP / ephemeral `ParentText` | Product: credential handoff to parents. Never audit/KV/argv. |
| Local plugin KV audit not tamper-evident | System admin / host root can rewrite KV. Operational log only. |
| Floating GitHub Action **major tags** | Fleet standard; CodeQL `actions/unpinned-tag` dismissed 2026-08-09. The reusable `plugin-ci` workflow's former `@main` pin was R1 and is now resolved. |
| Lab e2e not in default CI | Low ship volume; `AGENTS.md` / `docs/testing.md`. |
| Mattermost session CSRF | Platform; plugin does not add a second token. |
| Organizer is a delegated admin | Intended privilege (create users, reset in-scope passwords) is not a finding. |

## Audit history

### 2026-07-29 — Security audit stack (PRs #30–#35)

Tracking: [#36](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/36) Zen MCR. Authz, atomic KV, mmctl `--hashed`, slash audit/rate-limit, batch quota, ScopeEditor safety. Net improvement; no architectural auth bypass. Deferred: full create-user membership rollback.

### 2026-08-12 — IsSystemAdmin token match + Z3 alphabet

[#60](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/60) / [PR #61](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/61); [PR #59](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/59) Z3 disjointness.

### 2026-08-12 — Multi-model pass (Opus / Grok / Sol / GLM)

Scope: full tree at `chore/bump-js-yaml-4.3.1` (open PR #62 for js-yaml). Angles: supply-chain/CI (Opus brief + parent), adversarial exploit (Grok), integrity/races (Sol), control+ledger honesty (GLM).

**Result:** no remote unauthenticated RCE or organizer→`system_admin` escalation. Highest new item is **R2** (release `GITHUB_TOKEN` persisted into `.git/config` during `npm ci`). Next: V1, S1, I2, R1, R3, R4, C1–C3, P1.

Opus also **disproved**: js-yaml #62 is a real lockfile fix; Dependabot `ignore` does not suppress security updates; no XSS sink; `secrets: inherit` currently inherits nothing (keeps R1 Medium).

Sol I1 (config last-known-good) and I4 (reset TOCTOU) were filed High; parent **downgraded** to accepted residuals (#39 intent; concurrent-admin race). I5 matches #36 deferred rollback. Ledger C1 (Z3 not in CI) **renamed P1** so Opus C1 (no required checks) can keep its ID.

This file is the first in-repo ledger (previously implicit in `SECURITY.md` + closed issues). Wired into `.cursor/rules/security-audit.mdc`, `AGENTS.md`, and `CONTRIBUTING.md`. js-yaml CVE closed by PR #62.

### 2026-08-13 — V1 username argv hardening

`ValidateUsername` now requires a lowercase letter first, and the password
reset mmctl invocation uses `--` before the username. Table-driven Go tests
and the Z3 proof cover leading `--password`, `-h`, `--`, and digit-leading
usernames.

### 2026-08-12 — R4 npm override remediation

Pinned `brace-expansion` to 1.1.18 / 2.1.4 and `nanoid` to 3.3.17; refreshed `webapp/package-lock.json` and verified the resolved tree in [PR #65](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/pull/65).

### 2026-08-12 — R2/C3 release workflow remediation

Tracking: [#63](https://github.com/lucas-albers-lz4/mattermost-plugin-community-admin/issues/63). Release checkout no longer persists the write token, npm lifecycle scripts are disabled, and `go test ./server/...` gates `make dist`. Resolved R2 and C3; R3 remains open.

### 2026-08-12 — S1 elevated system role protection

`isProtected` now protects every elevated `system_*` role token while allowing the normal `system_user` and `system_guest` roles. Added a table-driven authorization matrix covering standalone and combined role lists.

### 2026-08-12 — I2 HTTP audit failure logging

HTTP `recordAudit` now logs `Record` errors using the same non-failing warning pattern as slash commands, preserving mutation availability while making audit loss observable.

### 2026-08-13 — R1/C2/P1 CI proof-gate remediation

Pinned the reusable `plugin-ci` workflow to commit
`86a78bd5bf6e009254bd0e23e316601d9d0db472`, removed the upstream-incompatible
nightly schedule, and wired `proof_username_whitelist.py` into a Z3-backed CI
job, the Makefile, and the staged-change pre-commit hook. C1 remains open
because branch protection is an operator setting; R3 remains open.

### 2026-08-12 — Low-severity hardening (V4, C4, E1–E3, I7, V5)

Admin list 500 responses now use the shared redacting error writer, and team
pagination avoids integer multiplication overflow. The API smoke test now
fails closed for the protected-target check; its login response uses a
restricted temporary file with cleanup, credentials are sent through stdin,
and the example endpoint uses a placeholder hostname. V5 is partially proven
by the existing production-shape argv unit test; live mmctl execution remains
manual.

## Review procedure

1. Read this file and [SECURITY.md](../SECURITY.md) first. Do not reopen [Accepted residuals](#accepted-residuals) or the Resolved table without new evidence.
2. Pick the surface with the **oldest date** in the coverage map. A pass that only re-reads `checker.go` is a pass that finds nothing new.
3. Diff the surface against [Controls in force](#controls-in-force). New mutator, route, KV write, or workflow step needs a named guard, a proof class, and a proof of that class.
4. Re-run the gates above. Apply the **false-green rule**. Check dismissed CodeQL alerts before filing.
5. File one tracking issue per theme with IDs (`V1`, `R1`, `I2`) matching this ledger. One issue plus one PR per hardening batch.
6. In the same PR: append a dated [Audit history](#audit-history) entry, refresh coverage-map dates, move closed rows to Resolved.
7. **Feature PR gate** (authz / password / audit / ScopeConfig / workflows): the same PR updates Controls in force for touched surfaces.
8. **Pre-merge:** PRs that add a mutator, mmctl argv change, or release workflow step get a security-review pass against this ledger before merge.

## Related

- [SECURITY.md](../SECURITY.md) — operator claims
- [design.md](design.md) — authz model, mmctl bridge
- [testing.md](testing.md) — pre-release lab gates
- [configuration.md](configuration.md) — organizer scope
- Open security work: GitHub `security` label when present
