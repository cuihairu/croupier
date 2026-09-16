import { defineConfig, devices } from '@playwright/test';

// 一次性线上验证配置：直连线上 dashboard，不起本地 webServer。
// 用法：cd web && npx playwright test --config=e2e-verify/playwright.config.ts
export default defineConfig({
  testDir: '.',
  workers: 1,
  retries: 0,
  timeout: 90000,
  reporter: [['list']],
  use: {
    ...devices['Desktop Chrome'],
    baseURL: process.env.BASE_URL || 'http://192.168.5.5:8000',
    locale: 'zh-CN',
    headless: true,
    actionTimeout: 20000,
    navigationTimeout: 45000,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
});
