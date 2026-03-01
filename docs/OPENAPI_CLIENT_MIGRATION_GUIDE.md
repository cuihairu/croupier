# OpenAPI Client Guide (No Legacy Fallback)

This document defines the client contract after the legacy descriptor fallback is removed.

## 1) API baseline

- Descriptor index: `GET /api/v1/functions/descriptors`
- Function OpenAPI operation: `GET /api/v1/functions/{id}/openapi`
- OpenAPI import: `POST /api/v1/functions/_import`

## 2) Required client behavior

1. Use descriptors only for list/index metadata (id/category/menu/risk).
2. Use OpenAPI operation as the runtime schema source.
3. Build input forms from:
   - `requestBody.content.application/json.schema`
   - `x-ui` (if present)
4. If OpenAPI is missing, show empty-state + retry/import guidance.
5. Do not fallback to legacy `params` or legacy UI schema.

## 3) Field mapping (descriptor/openapi extensions)

- `x-category` -> function category
- `x-risk` -> risk badge
- `x-entity` -> entity binding
- `x-operation` -> entity operation
- `x-ui` -> default UI schema

## 4) Validation checklist

- Function list page loads from descriptors.
- Function detail/invoke form renders from OpenAPI request schema.
- UI source only uses: `custom_metadata`, `config_file_override`, `openapi_x_ui`, `none`.
- Imported OpenAPI function appears in descriptors and detail API.
