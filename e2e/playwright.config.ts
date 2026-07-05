import path from 'path';

import dotenv from 'dotenv';
import {defineConfig} from '@playwright/test';

dotenv.config({path: path.join(__dirname, '.env')});

const testURL = process.env.TEST_URL || 'https://doomzilla.duckdns.org';

export default defineConfig({
    testDir: './tests',
    testIgnore: process.env.SCREENSHOTS ? [] : [/screenshots\.spec\.ts/],
    fullyParallel: false,
    forbidOnly: Boolean(process.env.CI),
    retries: process.env.CI ? 1 : 0,
    workers: 1,
    reporter: [['html', {open: 'never'}], ['list']],
    timeout: 90_000,
    use: {
        baseURL: testURL,
        channel: 'chrome',
        trace: 'off',
        screenshot: 'only-on-failure',
        video: 'off',
        storageState: {
            cookies: [],
            origins: [
                {
                    origin: testURL,
                    localStorage: [
                        {name: '__landingPageSeen__', value: 'true'},
                        {name: 'onboardingTaskListShow', value: 'false'},
                        {name: 'onboardingTaskListShowJoined', value: 'true'},
                        {name: 'was_logged_in', value: 'true'},
                    ],
                },
            ],
        },
    },
    outputDir: 'test-results',
});
