# Community Admin documentation screenshots

PNG assets for the [user guide](../../user-guide.md) and deploy-repo parent/ops docs. Regenerate from the plugin repo:

```sh
cd e2e
npm run screenshots
```

**Prerequisites:** test instance running, `e2e/.env` filled (`ORGANIZER_PASSWORD`), client IP in `TEST_ALLOWED_CIDR`.

**Theme:** Mattermost default/light. Re-capture after major Mattermost UI upgrades.

**Security:** credential banners are redacted before capture — passwords never appear in committed images.

| File | Caption | Used in |
|------|---------|---------|
| `01-channel-header.png` | Where to open Community Members (channel header or product menu) | user-guide, for-parents |
| `02-panel-list.png` | Community Members panel with member list | user-guide, README, 06-operations |
| `03-search-filter.png` | Search box filtering members | user-guide |
| `04-create-form.png` | Create user form | user-guide, for-parents |
| `05-credentials-create.png` | Credential handoff banner after create | user-guide, for-parents |
| `06-row-actions.png` | Reset password and remove-from-team buttons | user-guide |
| `07-credentials-reset.png` | Credential banner after password reset | user-guide |
| `08-slash-command.png` | Slash command typed in message box (mobile fallback) | user-guide |

Sync a parent-friendly subset to the deploy repo:

```sh
../mattermost-oci-deploy/scripts/sync-community-admin-screenshots.sh
```
