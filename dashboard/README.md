# Croupier Web Console

Frontend (Umi Max + Ant Design Pro) for Croupier — a function routing, job orchestration, and audit platform for game operations.

- Main Project: https://github.com/cuihairu/croupier
- This repo: Admin UI (menus, function invocation, jobs, audit, registry overview)

## Quick Start

Prerequisites
- Node.js 18+ (recommended)
- pnpm (or npm/yarn)

Install
```bash
pnpm install   # or: npm install / yarn
```

Dev Server
```bash
pnpm dev
# Serves at http://localhost:8000
# Proxies /api/* to http://localhost:8080 (configured in config/proxy.ts)
```

Build
```bash
pnpm build
# Output: dist/
# In production, Croupier Server can statically serve web/dist if present
```

Lint & Test
```bash
pnpm lint
pnpm test
```

## Backend Expectations
- Server API listens at 8080 by default; dev proxy rewrites legacy `/api/*` calls to `/api/v1/*`
- Auth endpoints used by UI:
  - POST /api/v1/auth/login -> { token, user }
  - GET  /api/v1/users/current -> { username, roles }
- Demo pages may call /api/rule (stubbed by Server for template compatibility)

Default Credentials (dev)
- username: `admin`
- password: `admin123`

## Notes
- This console is scaffolded from Ant Design Pro and customized for Croupier
- Branding and i18n strings are adjusted; further contributions welcome
- For deployment and core-server configuration, see the main repo

---

MIT License © Croupier
