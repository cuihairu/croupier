import { defineConfig, devices } from '@playwright/test';

const mockWebBaseURL = process.env.MOCK_DASHBOARD_WEB_BASE_URL || 'http://localhost:8000';
const realWebBaseURL = process.env.REAL_DASHBOARD_WEB_BASE_URL || 'http://localhost:8001';
const realServerBaseURL = process.env.REAL_DASHBOARD_SERVER_BASE_URL || 'http://localhost:28780';

function selectedProjects(argv: string[]): Set<string> {
  const result = new Set<string>();
  argv.forEach((arg, index) => {
    if (arg.startsWith('--project=')) {
      result.add(arg.slice('--project='.length));
      return;
    }
    if ((arg === '--project' || arg === '-p') && argv[index + 1]) {
      result.add(argv[index + 1]);
    }
  });
  return result;
}

function devServerPort(baseURL: string, fallback: number): string {
  const url = new URL(baseURL);
  if (url.port) return url.port;
  return url.protocol === 'https:' ? '443' : String(fallback);
}

const requestedProjects = selectedProjects(process.argv);
const startMockWeb = requestedProjects.size === 0 || requestedProjects.has('mock-dashboard');
const startRealWeb = requestedProjects.size === 0 || requestedProjects.has('real-dashboard');
const realDashboardScenarios =
  /@(?:fixture-health|sdk-|openapi-|schema-change|governance-change|stale-|safe-|identity-|republish-)/;

const webServers = [
  ...(startMockWeb
    ? [
        {
          command: `cross-env PORT=${devServerPort(mockWebBaseURL, 8000)} PLAYWRIGHT_BUNDLER=webpack REACT_APP_ENV=dev MOCK=all UMI_ENV=dev max dev`,
          url: mockWebBaseURL,
          reuseExistingServer: !process.env.CI,
          timeout: 180000,
        },
      ]
    : []),
  ...(startRealWeb
    ? [
        {
          command: `cross-env PORT=${devServerPort(realWebBaseURL, 8001)} PLAYWRIGHT_BUNDLER=webpack REACT_APP_ENV=dev MOCK=none UMI_ENV=dev CROUPIER_SERVER_BASE_URL=${realServerBaseURL} max dev`,
          url: realWebBaseURL,
          reuseExistingServer: !process.env.CI,
          timeout: 180000,
        },
      ]
    : []),
];

export default defineConfig({
  testDir: './e2e',
  globalSetup: './e2e/helpers/globalSetup.ts',
  globalTeardown: './e2e/helpers/globalTeardown.ts',
  fullyParallel: false, // 顺序执行以避免登录状态冲突
  forbidOnly: !!process.env.CI,
  // Dashboard scenarios mutate a shared fixture and must expose deterministic
  // failures. Retrying would mask ordering and lifecycle defects.
  retries: 0,
  workers: 1, // 单 worker 避免并发问题
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],
  timeout: process.env.CI ? 90000 : 60000, // CI 环境增加超时到 90 秒
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    headless: true,
    actionTimeout: process.env.CI ? 20000 : 15000, // CI 环境增加 action 超时
    navigationTimeout: process.env.CI ? 45000 : 30000, // CI 环境增加导航超时
  },
  projects: [
    {
      name: 'mock-dashboard',
      testIgnore: /fixture-health\.spec\.ts/,
      grepInvert: realDashboardScenarios,
      use: { ...devices['Desktop Chrome'], baseURL: mockWebBaseURL },
    },
    {
      // This project must always traverse the real Server API. Fixture setup
      // for this project is added separately and owns the Server/Agent data.
      name: 'real-dashboard',
      grep: realDashboardScenarios,
      use: { ...devices['Desktop Chrome'], baseURL: realWebBaseURL },
    },
  ],
  webServer: webServers,
});
