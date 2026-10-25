import { defineConfig } from '@playwright/test';
import fs from 'node:fs';
export default defineConfig({
  testDir:'./e2e', testMatch:process.env.CROSS_CLIENT==='1'?'**/cross-client.spec.ts':'**/live.spec.ts', timeout:120000, workers:1, retries:0,
  expect:{timeout:15000},
  outputDir:'test-results', reporter:'list',
  use:{baseURL:process.env.SETTLEGRAPH_WEB_URL??'http://127.0.0.1:3000',headless:true,
    viewport:{width:1440,height:960},trace:'off',screenshot:'only-on-failure',
    launchOptions:{executablePath:process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE??(fs.existsSync('/usr/bin/chromium')?'/usr/bin/chromium':undefined)}},
});
