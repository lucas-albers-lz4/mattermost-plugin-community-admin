import {expect, test} from '@playwright/test';

import {login, openCommunityMembersPanel, waitForPanelLoaded} from '../fixtures/auth';

const organizerUsername = process.env.ORGANIZER_USERNAME || 'test.organizer';
const organizerPassword = process.env.ORGANIZER_PASSWORD || '';
const nonOrganizerUsername = process.env.NON_ORGANIZER_USERNAME || 'testuser.beta';
const nonOrganizerPassword = process.env.NON_ORGANIZER_PASSWORD || '';
const memberUsername = process.env.MEMBER_USERNAME || 'testuser.alpha';

test.describe('Community Admin organizer panel', () => {
    test.beforeEach(async ({page}) => {
        test.skip(!organizerPassword, 'Set ORGANIZER_PASSWORD in e2e/.env');
        await login(page, organizerUsername, organizerPassword);
    });

    test('organizer_can_open_panel', async ({page}) => {
        await openCommunityMembersPanel(page);
        await waitForPanelLoaded(page);
        await expect(page.getByRole('heading', {name: 'Community Members'})).toBeVisible();
        await expect(page.getByTestId('community-admin-error')).toHaveCount(0);
    });

    test('organizer_lists_scoped_users', async ({page}) => {
        await openCommunityMembersPanel(page);
        await waitForPanelLoaded(page);
        await expect(page.getByTestId(`community-admin-user-row-${memberUsername}`)).toBeVisible();
        await expect(page.getByTestId('community-admin-user-row-testuser.beta')).toBeVisible();
    });

    test('organizer_creates_user', async ({page}) => {
        const username = `testuser.pw.${Date.now()}`;
        await openCommunityMembersPanel(page);
        await waitForPanelLoaded(page);

        await page.getByTestId('community-admin-create-toggle').click();
        await page.getByTestId('community-admin-create-username').fill(username);
        await page.getByTestId('community-admin-create-firstname').fill('Playwright');
        await page.getByTestId('community-admin-create-lastname').fill('Created');
        await page.getByTestId('community-admin-create-team').selectOption({label: 'test'});
        await page.getByTestId('community-admin-create-submit').click();

        await expect(page.getByTestId('community-admin-credentials')).toBeVisible({timeout: 15_000});
        const creds = await page.getByTestId('community-admin-credentials-text').textContent();
        expect(creds).toContain(username);
        expect(creds).toContain(process.env.TEST_URL?.replace(/^https?:\/\//, '') || 'doomzilla.duckdns.org');
        await expect(page.getByTestId(`community-admin-user-row-${username}`)).toBeVisible();
    });

    test('organizer_resets_password', async ({page}) => {
        await openCommunityMembersPanel(page);
        await waitForPanelLoaded(page);

        await page.getByTestId(`community-admin-reset-${memberUsername}`).click();
        await expect(page.getByTestId('community-admin-credentials')).toBeVisible({timeout: 15_000});
        const creds = await page.getByTestId('community-admin-credentials-text').textContent();
        expect(creds).toContain(memberUsername);
    });

    test('organizer_removes_from_team', async ({page}) => {
        const username = `testuser.pw.remove.${Date.now()}`;
        await openCommunityMembersPanel(page);
        await waitForPanelLoaded(page);

        await page.getByTestId('community-admin-create-toggle').click();
        await page.getByTestId('community-admin-create-username').fill(username);
        await page.getByTestId('community-admin-create-firstname').fill('Remove');
        await page.getByTestId('community-admin-create-lastname').fill('Me');
        await page.getByTestId('community-admin-create-team').selectOption({label: 'test'});
        await page.getByTestId('community-admin-create-submit').click();
        await expect(page.getByTestId(`community-admin-user-row-${username}`)).toBeVisible({timeout: 15_000});

        await page.getByTestId(`community-admin-remove-${username}`).click();
        await expect(page.getByTestId(`community-admin-user-row-${username}`)).toHaveCount(0, {timeout: 15_000});
    });
});

test.describe('Community Admin access gate', () => {
    test('non_organizer_no_panel', async ({page, request}) => {
        test.skip(!nonOrganizerPassword, 'Set NON_ORGANIZER_PASSWORD in e2e/.env');

        let betaPassword = nonOrganizerPassword;
        const probe = await request.post('/api/v4/users/login', {data: {login_id: nonOrganizerUsername, password: betaPassword}});
        if (!probe.ok()) {
            const orgLogin = await request.post('/api/v4/users/login', {data: {login_id: organizerUsername, password: organizerPassword}});
            const orgToken = orgLogin.headers()['token'];
            const users = await request.get('/plugins/com.lalbers.community-admin/api/v1/users?q=beta', {
                headers: {Authorization: `Bearer ${orgToken}`},
            });
            const betaId = (await users.json()).users?.find((u: {username: string}) => u.username === nonOrganizerUsername)?.id;
            const reset = await request.post(`/plugins/com.lalbers.community-admin/api/v1/users/${betaId}/reset-password`, {
                headers: {Authorization: `Bearer ${orgToken}`},
                data: {},
            });
            betaPassword = (await reset.json()).password || betaPassword;
        }

        await login(page, nonOrganizerUsername, betaPassword);

        const menuLabels = ['Product switch menu', 'Main Menu', "User's account menu"];
        for (const label of menuLabels) {
            const menu = page.getByRole('button', {name: new RegExp(label, 'i')}).first();
            if (await menu.count() === 0) {
                continue;
            }
            await menu.click();
            await expect(page.getByRole('button', {name: 'Community Members'})).toHaveCount(0);
            await page.keyboard.press('Escape').catch(() => undefined);
        }

        const opened = await page.evaluate(() => {
            if (typeof window.__communityAdminOpenPanel === 'function') {
                window.__communityAdminOpenPanel();
                return true;
            }
            return false;
        });
        if (opened) {
            await expect(page.getByTestId('community-admin-panel')).toHaveCount(0);
        }
    });
});
