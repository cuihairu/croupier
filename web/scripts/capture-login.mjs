/**
 * Capture the login page subtitle view (F5 evidence).
 * Usage: node --import tsx scripts/capture-login.mjs <outFile>
 */
import path from 'node:path';
import { chromium } from '@playwright/test';

const BASE = process.argv[2] || 'http://127.0.0.1:8000';
const OUT = path.resolve(process.argv[3]);

function resolveChromium() {
  const cacheRoot = path.join(process.env.HOME || '/root', '.cache', 'ms-playwright');
  let candidates = [];
  try {
    candidates = fs
      .readdirSync(cacheRoot)
      .filter((d) => d.startsWith('chromium-'))
      .sort()
      .reverse();
  } catch {}
  for (const dir of candidates) {
    for (const rel of ['chrome-linux64/chrome', 'chrome-linux/chrome']) {
      const exe = path.join(cacheRoot, dir, rel);
      if (fs.existsSync(exe)) return { executablePath: exe };
    }
  }
  return {};
}
import fs from 'node:fs';

async function main() {
  const browser = await chromium.launch(resolveChromium());
  const ctx = await browser.newContext({
    baseURL: BASE,
    locale: 'en-US',
    viewport: { width: 1440, height: 900 },
  });
  const page = await ctx.newPage();
  await page.goto('/user/login', { waitUntil: 'domcontentloaded' });
  await page.locator('form, .ant-pro-form').first().waitFor({ state: 'visible', timeout: 60000 });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: OUT, fullPage: false });
  await browser.close();
  console.log('saved', OUT);
}
main().catch((e) => {
  console.error(e);
  process.exit(1);
});
