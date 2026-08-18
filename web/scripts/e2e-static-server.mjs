/**
 * Static E2E server for the mock-dashboard project.
 *
 * Serves the production build (dist/) with SPA history fallback and mounts
 * the same umi mock handlers (mock/*.ts) used by `max dev`, so the mock
 * E2E suite can run against compiled assets with zero on-demand bundling.
 *
 * Usage: node scripts/e2e-static-server.mjs <distDir>
 * Env:   PORT (default 8000)
 */
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const root = path.resolve(process.argv[2] || 'dist');
const mockDir = path.resolve(import.meta.dirname, '..', 'mock');
const port = Number(process.env.PORT || 8000);

const types = {
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.html': 'text/html',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
  '.map': 'application/json',
  '.woff2': 'font/woff2',
};

/** Collect handler maps from every mock/*.ts|js module default export. */
async function loadMockHandlers() {
  const handlers = new Map();
  if (!fs.existsSync(mockDir)) return handlers;
  // tsx provides TS loading; fall back to plain .js modules when absent.
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

/** Normalize an incoming path against the express-style route table. */
function matchMock(handlers, method, pathname) {
  for (const [route, handler] of handlers) {
    // Route format: "METHOD /path/:param"（umi mock 允许省略 METHOD 前缀 = GET）
    let m = 'GET';
    let pattern = route.trim();
    const spaceIdx = pattern.indexOf(' ');
    if (spaceIdx > 0 && /^[A-Z]+$/.test(pattern.slice(0, spaceIdx))) {
      m = pattern.slice(0, spaceIdx);
      pattern = pattern.slice(spaceIdx + 1).trim();
    }
    if (m !== method) continue;
    // ':param' 段转正则
    const source = pattern
      .split('/')
      .map((seg) => (seg.startsWith(':') ? '([^/]+)' : seg.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
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

http.createServer((req, res) => {
  const url = new URL(req.url || '/', `http://127.0.0.1:${port}`);
  const pathname = decodeURIComponent(url.pathname);

  const hit = matchMock(handlers, req.method || 'GET', pathname);
  if (hit) {
    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
    });
    req.on('end', () => {
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
          res.writeHead(this.statusCode, { 'content-type': typeof payload === 'string' ? 'text/html; charset=utf-8' : 'application/json' });
          res.end(typeof payload === 'string' ? payload : JSON.stringify(payload));
        },
        json(payload) {
          res.writeHead(this.statusCode, { 'content-type': 'application/json' });
          res.end(JSON.stringify(payload));
        },
        setHeader(name, value) {
          res.setHeader(name, value);
        },
      };
      try {
        Promise.resolve(hit.handler(expressLikeReq, expressLikeRes)).catch(() => undefined);
      } catch {
        res.writeHead(500);
        res.end();
      }
    });
    return;
  }

  // Static assets with SPA fallback
  let file = path.join(root, pathname);
  if (!file.startsWith(root)) {
    res.writeHead(403);
    res.end();
    return;
  }
  if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) {
    file = path.join(root, 'index.html');
  }
  const ext = path.extname(file);
  res.writeHead(200, { 'content-type': types[ext] || 'application/octet-stream' });
  fs.createReadStream(file).pipe(res);
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
