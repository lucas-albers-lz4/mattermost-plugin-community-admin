import {APIRequestContext, expect, type Page} from '@playwright/test';

const baseURL = process.env.TEST_URL || 'https://doomzilla.duckdns.org';

export async function apiLogin(request: APIRequestContext, username: string, password: string): Promise<string> {
    const response = await request.post(`${baseURL}/api/v4/users/login`, {
        data: {login_id: username, password},
    });
    expect(response.ok()).toBeTruthy();
    const token = response.headers()['token'];
    if (!token) {
        throw new Error('login response missing token header');
    }
    return token;
}

export async function login(page: Page, username: string, password: string): Promise<void> {
    let lastError: unknown;
    for (let attempt = 0; attempt < 3; attempt++) {
        try {
            await page.goto('/login', {waitUntil: 'domcontentloaded', timeout: 60_000});
            await page.locator('#input_loginId, input[name="loginId"]').first().waitFor({timeout: 15_000});
            await page.locator('#input_loginId, input[name="loginId"]').first().fill(username);
            await page.locator('#input_password-input, input[type="password"]').first().fill(password);
            await page.getByRole('button', {name: /^Log in$/i}).click();
            await page.waitForURL(/\/channels\//, {timeout: 45_000});
            await waitForPluginLoaded(page);
            await dismissOnboarding(page);
            return;
        } catch (error) {
            lastError = error;
            await page.waitForTimeout(2_000);
        }
    }
    throw lastError;
}

export async function dismissOnboarding(page: Page): Promise<void> {
    const noThanks = page.getByText("No thanks, I'll figure it out myself");
    if (await noThanks.isVisible({timeout: 5_000}).catch(() => false)) {
        await noThanks.click({force: true});
    }

    const overlay = page.locator('[data-cy="onboarding-task-list-overlay"]');
    if (await overlay.isVisible({timeout: 2_000}).catch(() => false)) {
        await page.keyboard.press('Escape');
        await overlay.waitFor({state: 'hidden', timeout: 10_000}).catch(() => undefined);
    }

    if (await overlay.isVisible({timeout: 500}).catch(() => false)) {
        await page.evaluate(() => {
            document.querySelector('[data-cy="onboarding-task-list-overlay"]')?.remove();
        });
    }
}

export async function waitForPluginLoaded(page: Page): Promise<void> {
    await page.waitForResponse(
        (response) => response.url().includes('community-admin') && response.url().includes('bundle.js') && response.ok(),
        {timeout: 30_000},
    ).catch(() => undefined);
}

export async function openCommunityMembersPanel(page: Page): Promise<void> {
    await waitForPluginLoaded(page);
    await dismissOnboarding(page);

    const headerButton = page.getByRole('button', {name: 'Community Members'});
    if (await headerButton.first().isVisible({timeout: 2_000}).catch(() => false)) {
        await headerButton.first().click();
        await expect(page.getByTestId('community-admin-panel')).toBeVisible({timeout: 20_000});
        return;
    }

    const menuLabels = ['Product switch menu', 'Main Menu', "User's account menu"];
    for (const label of menuLabels) {
        const menu = page.getByRole('button', {name: new RegExp(label, 'i')}).first();
        if (await menu.count() === 0) {
            continue;
        }
        await menu.click({timeout: 5_000});
        const item = page.getByRole('button', {name: 'Community Members'}).or(page.getByText('Community Members', {exact: true}));
        if (await item.first().isVisible({timeout: 1_000}).catch(() => false)) {
            await item.first().click();
            await expect(page.getByTestId('community-admin-panel')).toBeVisible({timeout: 20_000});
            return;
        }
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
        await expect(page.getByTestId('community-admin-panel')).toBeVisible({timeout: 20_000});
        return;
    }

    throw new Error('Could not open Community Members panel');
}

export async function waitForPanelLoaded(page: Page): Promise<void> {
    await expect(page.getByTestId('community-admin-panel')).toBeVisible();
    await expect(page.getByTestId('community-admin-loading')).toHaveCount(0);
    await expect(page.getByTestId('community-admin-user-table')).toBeVisible();
}
