/**
 * real-dashboard fixture lifecycle管理。
 *
 * 命名 fixture "real-dashboard" 启动真实 Server + Agent + Go SDK + /players
 * OpenAPI provider，使用专用干净 scope（默认 e2e-game/e2e）。globalSetup 启动，
 * globalTeardown 通过 SIGTERM 让其仅清理自身 scope 后退出。
 */

import { spawn, spawnSync, type ChildProcess } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

export interface RealFixtureState {
  gameId: string;
  env: string;
  httpAddr: string;
  controlAddr: string;
  agentAddr: string;
  providerAddr: string;
  fixtureAddr: string;
  baseDir: string;
  serverBaseURL: string;
  fixtureBaseURL: string;
  providerBaseURL: string;
  pid: number;
}

const repoRoot = path.resolve(__dirname, '..', '..', '..');
const stateFile = path.resolve(__dirname, '..', '.real-fixture-state.json');
const binDir = path.resolve(__dirname, '..', '.fixture-bin');

function serverBaseURL(): string {
  return process.env.REAL_DASHBOARD_SERVER_BASE_URL || 'http://localhost:28780';
}

export function realFixtureStatePath(): string {
  return stateFile;
}

export function readRealFixtureState(): RealFixtureState {
  const raw = fs.readFileSync(stateFile, 'utf-8');
  return JSON.parse(raw) as RealFixtureState;
}

export function shouldStartRealFixture(): boolean {
  if (process.env.REAL_DASHBOARD_FIXTURE === '1') return true;
  if (process.env.REAL_DASHBOARD_FIXTURE === '0') return false;
  return process.argv.some((arg) => arg.includes('real-dashboard'));
}

async function waitFor(
  url: string,
  timeoutMs: number,
  validate?: (body: string) => boolean,
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  let lastError: unknown = null;
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(url);
      if (resp.ok) {
        const body = await resp.text();
        if (!validate || validate(body)) return;
        lastError = new Error(`unexpected body from ${url}: ${body.slice(0, 200)}`);
      } else {
        lastError = new Error(`HTTP ${resp.status} from ${url}`);
      }
    } catch (err) {
      lastError = err;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`timeout waiting for ${url}: ${String(lastError)}`);
}

export async function startRealFixture(): Promise<RealFixtureState> {
  fs.mkdirSync(binDir, { recursive: true });
  const serverBin = path.join(binDir, 'croupier-server');

  const build = spawnSync('go', ['build', '-o', serverBin, './cmd/server'], {
    cwd: repoRoot,
    stdio: 'inherit',
  });
  if (build.status !== 0) {
    throw new Error('failed to build croupier-server for real-dashboard fixture');
  }

  const httpURL = new URL(serverBaseURL());
  const httpAddr = `127.0.0.1:${httpURL.port || '28780'}`;

  const child: ChildProcess = spawn(
    serverBin,
    ['dev-fixture', '--http-addr', httpAddr, '--bootstrap-dir', path.join(repoRoot, 'configs')],
    { cwd: repoRoot, stdio: ['ignore', 'pipe', 'inherit'] },
  );

  let readyLine = '';
  const readyPromise = new Promise<RealFixtureState>((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error('fixture did not print FIXTURE_READY in 120s')),
      120000,
    );
    child.stdout?.on('data', (chunk: Buffer) => {
      const text = chunk.toString();
      process.stdout.write(text);
      readyLine += text;
      const idx = readyLine.indexOf('FIXTURE_READY ');
      if (idx >= 0) {
        const jsonStart = idx + 'FIXTURE_READY '.length;
        const jsonEnd = readyLine.indexOf('\n', jsonStart);
        if (jsonEnd > jsonStart) {
          clearTimeout(timer);
          const parsed = JSON.parse(readyLine.slice(jsonStart, jsonEnd)) as Omit<
            RealFixtureState,
            'serverBaseURL' | 'fixtureBaseURL' | 'providerBaseURL' | 'pid'
          >;
          resolve({
            ...parsed,
            serverBaseURL: `http://${parsed.httpAddr}`,
            fixtureBaseURL: `http://${parsed.fixtureAddr}`,
            providerBaseURL: `http://${parsed.providerAddr}`,
            pid: child.pid ?? 0,
          });
        }
      }
    });
    child.on('exit', (code) => {
      clearTimeout(timer);
      reject(new Error(`fixture process exited early with code ${code}`));
    });
  });

  const state = await readyPromise;
  await waitFor(`${state.serverBaseURL}/healthz`, 60000);
  await waitFor(`${state.fixtureBaseURL}/__fixture__/health`, 120000, (body) => {
    try {
      const parsed = JSON.parse(body) as { status?: string; agentConnected?: boolean };
      return parsed.status === 'ok' && parsed.agentConnected === true;
    } catch {
      return false;
    }
  });

  fs.writeFileSync(stateFile, JSON.stringify(state, null, 2));
  console.log(
    `[real-dashboard fixture] ready: server=${state.serverBaseURL} fixture=${state.fixtureBaseURL}`,
  );
  return state;
}

export async function stopRealFixture(): Promise<void> {
  if (!fs.existsSync(stateFile)) return;
  const state = readRealFixtureState();
  if (state.pid > 0) {
    try {
      process.kill(state.pid, 'SIGTERM');
    } catch {
      // already exited
    }
    const deadline = Date.now() + 30000;
    while (Date.now() < deadline) {
      try {
        process.kill(state.pid, 0);
        await new Promise((resolve) => setTimeout(resolve, 500));
      } catch {
        break;
      }
    }
  }
  fs.rmSync(stateFile, { force: true });
}
