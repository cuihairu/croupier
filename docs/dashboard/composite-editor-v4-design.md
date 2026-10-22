---
title: Composite editor V4 design
---

# Composite editor V4 design

V4 extends V3 with a reusable component-template layer. Functions remain atomic capabilities and pages remain the final composition; templates capture recurring combinations of functions, layout, actions, and links.

## Three-layer model

```text
Functions -> Component templates -> Pages
```

V4 removes the repeated work of rebuilding a management page from several functions. A template can include a table, detail modal, operation forms, row actions, and refresh behavior, then be instantiated and adjusted as ordinary editor nodes.

## Template model

| Field                  | Purpose                                                                |
| ---------------------- | ---------------------------------------------------------------------- |
| `key`                  | Stable unique template identity.                                       |
| `name` / `description` | Localized display metadata.                                            |
| `category`             | Operations, support, data, configuration, or custom.                   |
| `requiredFunctions`    | Scope availability check during insertion.                             |
| `tree`                 | Editor-node subtree with the same structure as a page.                 |
| `builtin`              | Marks templates generated from contracts; built-ins cannot be deleted. |
| `params`               | Optional display-property parameters with defaults.                    |

## Instantiation

1. Compare `requiredFunctions` with the functions available in the current scope.
2. Deep-copy the template tree.
3. Allocate fresh node identifiers.
4. Remap event, row-action, refresh, and parameter references.
5. Insert the subtree at the drop point or as root children when parameter collection is required.

Template instances are expanded rather than referenced. Operators can edit columns, titles, and local actions without changing every future page; this also avoids publication-time questions about template versions.

## Parameterization

Only safe display properties are parameterizable:

- `title`
- `span`
- `autoRun`

Execution-critical fields such as function IDs, row actions, events, and input mappings cannot be parameterized. Parameter keys must be unique and must point to an existing node in the template tree. Values are stored with the template and applied during instantiation; omitted values fall back to defaults.

## Built-in templates

The server scans complete resource contracts and generates management templates:

| Contract capability           | Generated node                          |
| ----------------------------- | --------------------------------------- |
| `collection_query`            | Auto-run, paginated table.              |
| `item_query`                  | Detail fields in a row-triggered modal. |
| `item_create` / `item_update` | Create or edit forms.                   |
| `item_delete`                 | Dangerous row action with confirmation. |

A complete CRUD resource becomes one management template rather than four unrelated fragments.

## API surface

```http
GET    /api/v1/component-templates
GET    /api/v1/component-templates/:key
POST   /api/v1/component-templates
PUT    /api/v1/component-templates/:key
DELETE /api/v1/component-templates/:key
POST   /api/v1/component-templates/regenerate
```

Availability is checked locally while loading the component library; a missing-function card is disabled with the missing IDs shown.

## Relationship to V3

V4 does not replace V3. It adds a component-library tab, reusable saved subtrees, contract-derived templates, and parameters. Compilation, validation, publication, and execution still operate on an ordinary expanded `PageNode` tree, so V3 behavior and tests remain the baseline.
