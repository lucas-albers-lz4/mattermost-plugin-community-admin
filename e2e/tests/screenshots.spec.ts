import {expect, test, type Page} from '@playwright/test';

import {apiLogin, dismissOnboarding, login, openCommunityMembersPanel, waitForPanelLoaded, waitForPluginLoaded} from '../fixtures/auth';
import {capture, DOC_VIEWPORT, highlight} from '../fixtures/screenshots';

const organizerUsername = process.env.ORGANIZER_USERNAME || 'test.organizer';
const organizerPassword = process.env.ORGANIZER_PASSWORD || '';
const memberUsername = process.env.MEMBER_USERNAME || 'testuser.alpha';
const demoUsername = 'doc.demo.user';

async function captureOpenPanelEntry(page: Page): Promise<void> {
    const headerButton = page.getByRole('button', {name: 'Community Members'}).first();
    if (await headerButton.isVisible({timeout: 2_000}).catch(() => false)) {
        const unhighlight = await highlight(headerButton);
        await capture(page, '01-channel-header.png', {fullPage: true});
        await unhighlight();
        return;
    }

    const menuLabels = ['Product switch menu', 'Main Menu', "User's account menu"];
    for (const label of menuLabels) {
        const menu = page.getByRole('button', {name: new RegExp(label, 'i')}).first();
        if ((await menu.count()) === 0) {
            continue;
        }
        await menu.click({timeout: 5_000});
        const menuItem = page.getByRole('button', {name: 'Community Members'})
            .or(page.getByText('Community Members', {exact: true}))
            .first();
        if (await menuItem.isVisible({timeout: 2_000}).catch(() => false)) {
            const unhighlight = await highlight(menuItem);
            await capture(page, '01-channel-header.png', {fullPage: true});
            await unhighlight();
            await page.keyboard.press('Escape').catch(() => undefined);
            return;
        }
        await page.keyboard.press('Escape').catch(() => undefined);
    }

    // Entry Edition test stacks may omit the menu item; show a doc callout on the channel view.
    await page.evaluate(() => {
        const banner = document.createElement('div');
        banner.id = 'doc-screenshot-callout';
        banner.textContent = 'Organizers: open Community Members from the channel header (users icon) or the product menu (grid icon, top left).';
        banner.style.cssText = [
            'position:fixed',
            'top:56px',
            'left:50%',
            'transform:translateX(-50%)',
            'z-index:99999',
            'background:#fff3cd',
            'border:2px solid #f5c518',
            'padding:12px 16px',
            'border-radius:8px',
            'max-width:640px',
            'font:14px/1.4 sans-serif',
            'box-shadow:0 2px 8px rgba(0,0,0,.15)',
        ].join(';');
        document.body.appendChild(banner);
    });
    await capture(page, '01-channel-header.png', {fullPage: true});
    await page.evaluate(() => document.getElementById('doc-screenshot-callout')?.remove());
}

async function openPanelQuick(page: Page): Promise<void> {
    await waitForPluginLoaded(page);
    await dismissOnboarding(page);

    const headerButton = page.getByRole('button', {name: 'Community Members'}).first();
    if (await headerButton.isVisible({timeout: 2_000}).catch(() => false)) {
        await headerButton.click();
        await expect(page.getByTestId('community-admin-panel')).toBeVisible({timeout: 20_000});
        return;
    }

    const opened = await page.evaluate(() => {
        if (typeof window.__communityAdminOpenPanel === 'function') {
            window.__communityAdminOpenPanel();
            return true;
        }
        return false;
    });
    if (opened) {
        await expect(page.getByTestId('community-admin-panel')).toBeVisible({timeout: 20_000});
        return;
    }

    await openCommunityMembersPanel(page);
}

test.describe.configure({mode: 'serial'});

test.describe('@screenshots Documentation captures', () => {
    test.beforeAll(() => {
        test.skip(!organizerPassword, 'Set ORGANIZER_PASSWORD in e2e/.env');
    });

    test('capture documentation screenshots', async ({page, request}) => {
        test.setTimeout(180_000);
        await page.setViewportSize(DOC_VIEWPORT);
        await login(page, organizerUsername, organizerPassword);

        // 01 — where to open the panel (channel header or product menu on Entry Edition)
        await waitForPluginLoaded(page);
        await dismissOnboarding(page);
        await captureOpenPanelEntry(page);

        // 02 — panel member list
        await openPanelQuick(page);
        await waitForPanelLoaded(page);
        await expect(page.getByTestId(`community-admin-user-row-${memberUsername}`)).toBeVisible();
        await capture(page, '02-panel-list.png');

        // 03 — search filter
        await page.getByTestId('community-admin-search').fill('alpha');
        await page.getByTestId('community-admin-search-btn').click();
        await expect(page.getByTestId(`community-admin-user-row-${memberUsername}`)).toBeVisible();
        await capture(page, '03-search-filter.png');

        // 04 — create user form
        await page.getByTestId('community-admin-search').fill('');
        await page.getByTestId('community-admin-search-btn').click();
        await page.getByTestId('community-admin-create-toggle').click();
        await expect(page.getByTestId('community-admin-create-form')).toBeVisible();
        await capture(page, '04-create-form.png', {
            locator: page.getByTestId('community-admin-panel'),
        });

        // 05 — credentials after create (idempotent)
        const demoRow = page.getByTestId(`community-admin-user-row-${demoUsername}`);
        let credentialsUsername = demoUsername;
        if (await demoRow.count() === 0) {
            await page.getByTestId('community-admin-create-username').fill(demoUsername);
            await page.getByTestId('community-admin-create-firstname').fill('Doc');
            await page.getByTestId('community-admin-create-lastname').fill('Demo');
            await page.getByTestId('community-admin-create-team').selectOption({label: 'test'});
            await page.getByTestId('community-admin-create-submit').click();
            if (await demoRow.isVisible({timeout: 5_000}).catch(() => false)) {
                credentialsUsername = demoUsername;
            } else {
                credentialsUsername = memberUsername;
                await page.getByTestId(`community-admin-reset-${memberUsername}`).click();
            }
        } else {
            await page.getByTestId(`community-admin-reset-${demoUsername}`).click();
        }
        await expect(page.getByTestId('community-admin-credentials')).toBeVisible({timeout: 15_000});
        await capture(page, '05-credentials-create.png', {
            locator: page.getByTestId('community-admin-panel'),
            redactCredentials: true,
            username: credentialsUsername,
        });

        // 06 — row actions on a member
        const dismissButton = page.getByTestId('community-admin-credentials').getByRole('button', {name: 'Dismiss'});
        if (await dismissButton.isVisible().catch(() => false)) {
            await dismissButton.click();
        }
        const memberRow = page.getByTestId(`community-admin-user-row-${memberUsername}`);
        await expect(memberRow).toBeVisible();
        await capture(page, '06-row-actions.png', {locator: memberRow});

        // 07 — credentials after reset
        await page.getByTestId(`community-admin-reset-${memberUsername}`).click();
        await expect(page.getByTestId('community-admin-credentials')).toBeVisible({timeout: 15_000});
        await capture(page, '07-credentials-reset.png', {
            locator: page.getByTestId('community-admin-panel'),
            redactCredentials: true,
            username: memberUsername,
        });

        // 08 — slash command in composer
        await page.getByTestId('community-admin-close').click();
        await expect(page.getByTestId('community-admin-panel')).toHaveCount(0, {timeout: 10_000});
        const composer = page.locator('#post_textbox, [data-testid="post_textbox"], [role="textbox"][aria-label*="message" i]').first();
        await composer.waitFor({state: 'visible', timeout: 15_000});
        await composer.click();
        await composer.fill(`/community-admin reset-password ${memberUsername}`);
        await capture(page, '08-slash-command.png', {fullPage: false});

        // Cleanup demo user from team when possible
        const token = await apiLogin(request, organizerUsername, organizerPassword);
        const usersResponse = await request.get(`/plugins/com.lalbers.community-admin/api/v1/users?q=${demoUsername}`, {
            headers: {Authorization: `Bearer ${token}`},
        });
        if (usersResponse.ok()) {
            const users = (await usersResponse.json()).users || [];
            const demoUser = users.find((user: {username: string}) => user.username === demoUsername);
            const meResponse = await request.get('/plugins/com.lalbers.community-admin/api/v1/me', {
                headers: {Authorization: `Bearer ${token}`},
            });
            if (demoUser?.id && meResponse.ok()) {
                const teamId = (await meResponse.json()).teams?.[0]?.id;
                if (teamId) {
                    await request.delete(`/plugins/com.lalbers.community-admin/api/v1/users/${demoUser.id}/teams/${teamId}`, {
                        headers: {Authorization: `Bearer ${token}`},
                    });
                }
            }
        }
    });
});
