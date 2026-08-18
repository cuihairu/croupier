/**
 * Static E2E server for the mock-dashboard project.
 *
 * Serves the production build (dist/) via serve-handler (SPA fallback,
 * correct mime/streaming) and mounts the umi mock handlers (mock/*.ts)
 * so the mock E2E suite runs against compiled assets with zero
 * on-demand bundling.
 *
 * Usage: node scripts/e2e-static-server.mjs <distDir>
 * Env:   PORT (default 8000)
 */
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import serveHandler from 'serve-handler';

const root = path.resolve(process.argv[2] || 'dist');
const mockDir = path.resolve(import.meta.dirname, '..', 'mock');
const port = Number(process.env.PORT || 8000);

/** Collect handler maps from every mock/*.ts|js module default export. */
async function loadMockHandlers() {
  const handlers = new Map();
  if (!fs.existsSync(mockDir)) return handlers;
  const files = fs
    .readdirSync(mockDir)
    .filter((f) => /\.(ts|js)$/.test(f) && !f.endsWith('.d.ts'))
    .sort();
  for (const file of files) {
    try {
      const mod = await import(`${pathToFileURL(path.join(mockDir, file)).href}?t=${Date.now()}`);
      const map = mod.default;
      if (map && typeof map === 'object') {
        for (const [route, handler] of Object.entries(map)) {
          if (typeof handler === 'function') handlers.set(route, handler);
        }
      }
    } catch {
      // Skip mock modules that fail to load outside the umi mock context.
    }
  }
  return handlers;
}

/** Match an incoming request against the express-style route table. */
function matchMock(handlers, method, pathname) {
  for (const [route, handler] of handlers) {
    let m = 'GET';
    let pattern = route.trim();
    const spaceIdx = pattern.indexOf(' ');
    if (spaceIdx > 0 && /^[A-Z]+$/.test(pattern.slice(0, spaceIdx))) {
      m = pattern.slice(0, spaceIdx);
      pattern = pattern.slice(spaceIdx + 1).trim();
    }
    if (m !== method) continue;
    const source = pattern
      .split('/')
      .map((seg) =>
        seg.startsWith(':') ? '([^/]+)' : seg.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'),
      )
      .join('/');
    const match = pathname.match(new RegExp(`^${source}/?$`));
    if (match) {
      const names = pattern.split('/').filter((s) => s.startsWith(':')).map((s) => s.slice(1));
      const params = {};
      names.forEach((name, i) => {
        params[name] = decodeURIComponent(match[i + 1]);
      });
      return { handler, params };
    }
  }
  return null;
}

const handlers = await loadMockHandlers();
console.log(`[e2e-static] mock handlers: ${handlers.size}, dist: ${root}`);

function readBody(req) {
  return new Promise((resolve) => {
    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
    });
    req.on('end', () => resolve(body));
    req.on('error', () => resolve(body));
  });
}

http.createServer(async (req, res) => {
  const url = new URL(req.url || '/', `http://127.0.0.1:${port}`);
  const pathname = decodeURIComponent(url.pathname);

  const hit = matchMock(handlers, req.method || 'GET', pathname);
  if (hit) {
    const body = await readBody(req);
    let sent = false;
    const finish = (code, payload, type) => {
      if (sent) return;
      sent = true;
      res.writeHead(code, { 'content-type': type });
      res.end(payload);
    };
    const expressLikeReq = {
      method: req.method,
      url: req.url,
      params: hit.params,
      query: Object.fromEntries(url.searchParams),
      body: body ? safeJSON(body) : {},
      headers: req.headers,
    };
    const expressLikeRes = {
      statusCode: 200,
      status(code) {
        this.statusCode = code;
        return this;
      },
      send(payload) {
        finish(
          this.statusCode,
          typeof payload === 'string' ? payload : JSON.stringify(payload),
          typeof payload === 'string' ? 'text/html; charset=utf-8' : 'application/json',
        );
      },
      json(payload) {
        finish(this.statusCode, JSON.stringify(payload), 'application/json');
      },
      setHeader(name, value) {
        res.setHeader(name, value);
      },
    };
    try {
      await Promise.race([
        Promise.resolve(hit.handler(expressLikeReq, expressLikeRes)),
        new Promise((_, reject) => setTimeout(() => reject(new Error('mock handler timeout')), 10000)),
      ]);
    } catch {
      finish(500, JSON.stringify({ error: 'mock_handler_failed' }), 'application/json');
    }
    if (!sent) finish(200, '{}', 'application/json');
    return;
  }

  // SPA fallback：请求路径在 dist 中不是真实文件时回退 index.html。
  // 注意 pageKey 形如 operation--mail.send（含点），不能只看扩展名，
  // 必须检查文件是否真实存在。
  let candidate = path.join(root, pathname);
  if (
    !candidate.startsWith(root) ||
    !fs.existsSync(candidate) ||
    fs.statSync(candidate).isDirectory()
  ) {
    candidate = path.join(root, 'index.html');
  }
  if (candidate.endsWith('index.html')) {
    res.writeHead(200, { 'content-type': 'text/html' });
    fs.createReadStream(candidate).pipe(res);
    return;
  }
  await serveHandler(req, res, { public: root });
}).listen(port, '127.0.0.1', () => {
  console.log(`[e2e-static] listening on http://127.0.0.1:${port}`);
});

function safeJSON(text) {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}
