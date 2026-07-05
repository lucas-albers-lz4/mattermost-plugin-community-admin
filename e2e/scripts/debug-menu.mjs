import dotenv from 'dotenv';
import path from 'path';
import {chromium} from '@playwright/test';

import {fileURLToPath} from 'url';

dotenv.config({path: path.join(path.dirname(fileURLToPath(import.meta.url)), '../.env')});

const baseURL = process.env.TEST_URL || 'https://doomzilla.duckdns.org';
const username = process.env.ORGANIZER_USERNAME || 'test.organizer';
const password = process.env.ORGANIZER_PASSWORD || '';

const browser = await chromium.launch({channel: 'chrome', headless: true});
const context = await browser.newContext({
    baseURL,
    storageState: {
        cookies: [],
        origins: [{origin: baseURL, localStorage: [{name: '__landingPageSeen__', value: 'true'}]}],
    },
});
const page = await context.newPage();

page.on('console', (msg) => console.log('CONSOLE', msg.type(), msg.text()));
page.on('pageerror', (err) => console.log('PAGEERROR', err.message));
page.on('requestfailed', (req) => console.log('REQFAIL', req.url(), req.failure()?.errorText));

await page.goto('/login');
await page.locator('#input_loginId').fill(username);
await page.locator('#input_password-input').fill(password);
await page.getByRole('button', {name: /^Log in$/i}).click();
await page.waitForURL(/\/channels\//, {timeout: 45_000});

const bundleResp = await page.waitForResponse(
    (r) => r.url().includes('community-admin') && r.url().includes('bundle.js'),
    {timeout: 30_000},
).catch(() => null);
console.log('bundle response', bundleResp?.status(), bundleResp?.url());

const dismiss = page.getByText("No thanks, I'll figure it out myself");
if (await dismiss.isVisible({timeout: 3000}).catch(() => false)) {
    await dismiss.click({force: true});
}
const overlay = page.locator('[data-cy="onboarding-task-list-overlay"]');
if (await overlay.isVisible({timeout: 2000}).catch(() => false)) {
    await page.keyboard.press('Escape');
    await overlay.waitFor({state: 'hidden', timeout: 10000}).catch(() => undefined);
}
if (await overlay.isVisible({timeout: 500}).catch(() => false)) {
    await page.evaluate(() => document.querySelector('[data-cy="onboarding-task-list-overlay"]')?.remove());
}

const buttons = await page.locator('button[aria-label]').evaluateAll((els) =>
    els.map((el) => el.getAttribute('aria-label')).filter(Boolean),
);
console.log('aria-label buttons:', buttons);

for (const label of ['Product switch menu', "User's account menu", 'Settings', 'Main Menu']) {
    const btn = page.getByRole('button', {name: new RegExp(label, 'i')}).first();
    if ((await btn.count()) === 0) {
        continue;
    }
    await btn.click({timeout: 5000});
    await page.waitForTimeout(500);
    const menuText = await page.locator('[role="menu"], .a11y__popup, #root-portal').last().innerText().catch(() => '');
    console.log(`--- menu "${label}" ---`);
    console.log(menuText.split('\n').slice(0, 30).join('\n'));
    console.log('has Community Members:', menuText.includes('Community Members'));
    await page.keyboard.press('Escape');
    await page.waitForTimeout(300);
}

const pluginRegistered = await page.evaluate(() => {
    return typeof window.registerPlugin === 'function';
});
console.log('registerPlugin exists:', pluginRegistered);

const pluginState = await page.evaluate(() => {
    const root = document.getElementById('root');
    if (!root) {
        return {error: 'no root'};
    }
    const fiberKey = Object.keys(root).find((k) => k.startsWith('__reactFiber$') || k.startsWith('__reactContainer$'));
    if (!fiberKey) {
        return {error: 'no fiber', keys: Object.keys(root).slice(0, 10)};
    }
    let fiber = root[fiberKey];
    for (let i = 0; i < 50 && fiber; i++) {
        const props = fiber.memoizedProps || fiber.pendingProps;
        if (props?.store?.getState) {
            const state = props.store.getState();
            const plugins = state.plugins?.components?.MainMenu || state.plugins?.components?.ProductMenuSwitcher || state.plugins?.components;
            return {
                pluginComponentKeys: plugins ? Object.keys(plugins) : [],
                mainMenu: state.plugins?.components?.MainMenu,
                productSwitcher: state.plugins?.components?.ProductMenuSwitcher || state.plugins?.components?.ProductSwitcher,
            };
        }
        fiber = fiber.return;
    }
    return {error: 'store not found'};
});
console.log('plugin redux state:', JSON.stringify(pluginState, null, 2));

await browser.close();
