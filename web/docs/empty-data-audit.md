# Empty Data Audit

Updated: 2026-03-17

## Goal

Track pages that can show empty or misleading data because the backend endpoint is not fully implemented, returns placeholder data, or still uses transitional behavior.

This file is intentionally operational. It is not a product spec.

## Confirmed High Risk Areas

### Storage

- Frontend page: `src/pages/Storage/index.tsx`
- Backend: `internal/api/storage/service.go`
- Status:
  - `SignedUrl` implemented
  - `ListObjects` implemented
  - `UploadObject` implemented
  - `DeleteObject` implemented
  - `BatchDeleteObjects` implemented
  - `RenameDirectory` implemented
- Current handling:
  - Frontend has been normalized to a single service layer.
  - Backend now supports real object-store operations and multipart upload.
  - Remaining risk is deployment config, not placeholder logic.

### Telemetry Dashboard

- Frontend page: `src/pages/Telemetry/index.tsx`
- Backend: `internal/api/ops/config` for external observability entry config
- Status:
  - in-page random metrics have been removed
  - page is now an external observability entry screen
  - real telemetry still depends on external platforms
- Current handling:
  - Warning banner added in page
  - Page reads external platform URLs from `/api/v1/ops/config`
  - Real telemetry access should use external Grafana / Jaeger / Alertmanager links

### Function Instances Detail / Debug

- Frontend page: `src/pages/Functions/Instances/index.tsx`
- Backend:
  - list comes from real registry endpoints
  - detail metrics / logs / debug execution have no real backend API yet
- Status:
  - instance list is real
  - detail metrics were previously generated in-browser
  - logs were previously generated in-browser
  - debug result was previously generated in-browser
- Current handling:
  - warning banner added in page
  - fake metrics/logs/debug success have been removed
  - page now explicitly shows transitional placeholders instead of fabricated runtime data

### Ops Services

- Frontend page: `src/pages/Ops/Services/index.tsx`
- Backend: `internal/api/ops/helpers.go`
- Status:
  - `opsServices` returns empty placeholder data
- Current handling:
  - Route hidden from menu
  - Direct access shows warning banner

### Ops Health

- Frontend page: `src/pages/Ops/Health/index.tsx`
- Backend: `internal/api/ops/helpers.go`
- Status:
  - `opsHealthGet` returns empty checks
  - `opsHealthRun` returns empty results
  - `opsHealthUpdate` returns placeholder success
- Current handling:
  - Route hidden from menu
  - Direct access shows warning banner

### Ops MQ

- Frontend page: `src/pages/Ops/MQ/index.tsx`
- Backend: `internal/api/ops/helpers.go`
- Status:
  - `opsMQ` returns empty queues
- Current handling:
  - Route hidden from menu
  - Direct access shows warning banner

### Ops Maintenance

- Frontend page: `src/pages/Ops/Maintenance/index.tsx`
- Backend: `internal/api/ops/helpers.go`
- Status:
  - `opsMaintenanceGet` returns empty windows
  - `opsMaintenanceUpdate` returns placeholder success
- Current handling:
  - Route hidden from menu
  - Direct access shows warning banner

### Function UI History / Rollback

- Frontend:
  - `src/components/FunctionUIManager/index.tsx`
  - `src/pages/Functions/SchemaDesigner/index.tsx`
- Backend: `internal/api/function/helpers.go`
- Status:
  - `GetUI` placeholder source detail exists
  - `SaveUI` metadata fallback only
  - `GetUIHistory` returns empty
  - `RollbackUI` not implemented
- Current handling:
  - Warning banner added in UI manager and schema designer

## Transitional But Usable Areas

### Ops Notifications

- Backend currently uses extension installation config as fallback storage.
- It is not fully native, but it is not purely empty placeholder behavior.
- Keep exposed for now.

### Ops Alerts / Backups / Notifications Domain Pages

- These have been redirected into extension domain entry pages instead of legacy ops pages.
- Continue extension-first model.

## Frontend Contract Cleanup Progress

Completed:

- Auth/profile/game/permission/player/node/alert/platform service layers no longer rely on old response envelopes.
- Analytics filters page fixed to use real `gameId` scope instead of numeric database `id`.
- Storage page now uses unified storage service functions.
- Several mobile layout issues fixed for:
  - profile
  - login
  - storage
  - game selector
  - analytics filters

Remaining focus:

- Continue scanning pages that still depend on backend placeholder endpoints.
- Prefer hiding or warning before exposing incomplete pages.

## Recommended Next Steps

1. Decide whether telemetry dashboard should be removed, hidden, or wired to a real backend API.
2. Implement real instance metrics/logs/debug APIs, or keep those tabs hidden until available.
3. Decide whether ops placeholder pages should be implemented or removed entirely.
4. Replace function UI placeholder history/rollback with real persistence.
5. Keep this file updated whenever a placeholder page is hidden, warned, or implemented.
