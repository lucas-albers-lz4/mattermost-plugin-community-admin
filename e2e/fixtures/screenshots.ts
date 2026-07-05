import fs from 'fs';
import path from 'path';

import type {Locator, Page} from '@playwright/test';

export const DOC_VIEWPORT = {width: 1440, height: 900};
export const OUTPUT_DIR = path.join(__dirname, '../../docs/images/community-admin');

export async function ensureOutputDir(): Promise<void> {
    await fs.promises.mkdir(OUTPUT_DIR, {recursive: true});
}

export type CaptureOptions = {
    locator?: Locator;
    fullPage?: boolean;
    redactCredentials?: boolean;
    username?: string;
};

export function demoCredentialText(username: string): string {
    const siteHost = (process.env.TEST_URL || 'https://doomzilla.duckdns.org').replace(/^https?:\/\//, '');
    return `Site: ${siteHost}\nUsername: ${username}\nPassword: (example only — share privately)`;
}

export async function redactCredentials(page: Page, username: string): Promise<void> {
    const demoText = demoCredentialText(username);
    await page.evaluate((text) => {
        const el = document.querySelector('[data-testid="community-admin-credentials-text"]');
        if (el) {
            el.textContent = text;
        }
    }, demoText);
}

export async function highlight(locator: Locator): Promise<() => Promise<void>> {
    if ((await locator.count()) === 0) {
        return async () => undefined;
    }

    await locator.first().evaluate((el) => {
        el.setAttribute('data-screenshot-highlight', 'true');
        (el as HTMLElement).style.outline = '3px solid #f5c518';
        (el as HTMLElement).style.outlineOffset = '2px';
    });

    return async () => {
        await locator.first().evaluate((el) => {
            el.removeAttribute('data-screenshot-highlight');
            (el as HTMLElement).style.outline = '';
            (el as HTMLElement).style.outlineOffset = '';
        }).catch(() => undefined);
    };
}

export async function capture(page: Page, filename: string, options: CaptureOptions = {}): Promise<string> {
    await ensureOutputDir();

    if (options.redactCredentials) {
        await redactCredentials(page, options.username || 'testuser.alpha');
    }

    const outputPath = path.join(OUTPUT_DIR, filename);
    const screenshotOptions = {
        path: outputPath,
        animations: 'disabled' as const,
        fullPage: options.fullPage ?? false,
    };

    if (options.locator) {
        await options.locator.screenshot(screenshotOptions);
    } else {
        await page.screenshot(screenshotOptions);
    }

    return outputPath;
}
