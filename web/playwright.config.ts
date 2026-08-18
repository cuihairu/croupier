import fs from 'node:fs';
import path from 'node:path';
import { defineConfig, devices } from '@playwright/test';

const mockWebBaseURL = process.env.MOCK_DASHBOARD_WEB_BASE_URL || 'http://localhost:8000';
const realWebBaseURL = process.env.REAL_DASHBOARD_WEB_BASE_URL || 'http://localhost:8001';
const realServerBaseURL = process.env.REAL_DASHBOARD_SERVER_BASE_URL || 'http://localhost:28780';

function mockAuthStateFile(): string | undefined {
  const statePath = path.resolve(__dirname, 'e2e', '.auth', 'mock.json');
  return fs.existsSync(statePath) ? statePath : undefined;
}

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

// Mock 套件在 CI 用生产构建（无按需编译、无 HMR 开销）；本地默认 dev
// （迭代快），可 MOCK_E2E_STATIC=1 强制静态。
const mockStatic = process.env.CI || process.env.MOCK_E2E_STATIC === '1';
// 本地静态模式且无 dist 时自动构建；CI 由 workflow 显式构建（日志可见）
const needLocalBuild = mockStatic && !fs.existsSync(path.resolve(__dirname, 'dist'));

const webServers = [
  ...(startMockWeb && mockStatic
    ? [
        {
          command: needLocalBuild
            ? `cross-env MOCK=all pnpm exec max build > /dev/null 2>&1 && cross-env PORT=${devServerPort(mockWebBaseURL, 8000)} node --import tsx scripts/e2e-static-server.mjs dist`
            : `cross-env PORT=${devServerPort(mockWebBaseURL, 8000)} node --import tsx scripts/e2e-static-server.mjs dist`,
          url: mockWebBaseURL,
          reuseExistingServer: false,
          timeout: 600000,
        },
      ]
    : []),
  ...(startMockWeb && !mockStatic
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
      use: {
        ...devices['Desktop Chrome'],
        baseURL: mockWebBaseURL,
        // globalSetup 登录一次并持久化 token，用例直接以认证态启动
        storageState: mockAuthStateFile(),
      },
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
