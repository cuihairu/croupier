import { defineConfig, devices } from '@playwright/test';

const mockWebBaseURL = process.env.MOCK_DASHBOARD_WEB_BASE_URL || 'http://localhost:8000';
const realWebBaseURL = process.env.REAL_DASHBOARD_WEB_BASE_URL || 'http://localhost:8001';
const realServerBaseURL = process.env.REAL_DASHBOARD_SERVER_BASE_URL || 'http://localhost:28780';

export default defineConfig({
  testDir: './e2e',
  globalSetup: './e2e/helpers/globalSetup.ts',
  globalTeardown: './e2e/helpers/globalTeardown.ts',
  fullyParallel: false, // 顺序执行以避免登录状态冲突
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1, // 单 worker 避免并发问题
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],
  timeout: 60000, // 每个测试 60 秒超时
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    headless: true,
    actionTimeout: 15000,
    navigationTimeout: 30000,
  },
  projects: [
    {
      name: 'mock-dashboard',
      use: { ...devices['Desktop Chrome'], baseURL: mockWebBaseURL },
    },
    {
      // This project must always traverse the real Server API. Fixture setup
      // for this project is added separately and owns the Server/Agent data.
      name: 'real-dashboard',
      use: { ...devices['Desktop Chrome'], baseURL: realWebBaseURL },
    },
  ],
  webServer: [
    {
      command: 'cross-env REACT_APP_ENV=dev MOCK=all UMI_ENV=dev max dev --port 8000',
      url: mockWebBaseURL,
      reuseExistingServer: !process.env.CI,
      timeout: 180000,
    },
    {
      command: `cross-env REACT_APP_ENV=dev MOCK=none UMI_ENV=dev CROUPIER_SERVER_BASE_URL=${realServerBaseURL} max dev --port 8001`,
      url: realWebBaseURL,
      reuseExistingServer: !process.env.CI,
      timeout: 180000,
    },
  ],
});
