---
title: Resource Catalog API
icon: cubes
order: 98
category:
  - API Reference
tag:
  - ResourceCapability
  - CapabilitySemantics
  - PageProposal
---

# Resource Catalog API

> **Status**: Current -- routes are registered in `internal/handler/routes.go`. Resources are the domain objects behind CRUD and non-CRUD page generation; the source of truth is the persisted FunctionContract, ResourceCapability, and CapabilitySemantics.

The Resource Catalog reads scoped, persisted FunctionContract, ResourceCapability, and CapabilitySemantics, helping users understand why the platform can or cannot generate default pages.

Note the division between the two prefixes: `/api/v1/resource-catalog` is the semantics governance API; `/api/v1/resources` is the read-only resource aggregation on the function-catalog side (`GET /api/v1/resources`, `GET /api/v1/resources/{resourceKey}`, `GET /api/v1/resources/{resourceKey}/operations`).

## Resource List and Details

```http
GET /api/v1/resource-catalog
GET /api/v1/resource-catalog/{resourceKey}
```

```ts
interface ResourceCatalogItem {
  resourceKey: string;
  status: "identified" | "pending";
  functions: Array<{ functionId: string; capability: string; source: string }>;
  semantics?: {
    source: string; // sdk_explicit | openapi_rest | platform_review
    hasIdentity: boolean;
    identityField?: string;
    hasCollection: boolean;
    hasCreate: boolean;
    hasUpdate: boolean;
    hasDelete: boolean;
  };
  diagnostics: Diagnostic[]; // e.g. resource_identity_not_verifiable
}
```

The detail view must show:

- The function bindings of the collection, item, create, update, delete, action, task, and report capabilities.
- Identity, collection response, identification source, confidence, source digest, and diagnostics.
- Which capabilities come from OpenAPI REST, an explicit SDK capability, or an administrator supplement.
- Which current Proposals and Published pages are affected.

## Semantics Editing and Versioning

```http
PUT  /api/v1/resource-catalog/{resourceKey}/semantics
GET  /api/v1/resource-catalog/{resourceKey}/semantics/versions
```

`PUT /semantics` edits only capability semantics (identity, lifecycle binding, action subject, task/report semantic), not PageSpec. Any semantic supplement requires versioning, permissions, audit, and OTel, and triggers a rebuild of affected Proposals.

## Semantic Conflicts and Adjudication

```http
GET  /api/v1/resource-catalog/{resourceKey}/conflicts
POST /api/v1/resource-catalog/{resourceKey}/conflicts/{field}/resolve
```

When multi-source semantics (`platform_review > sdk_explicit > openapi_rest`) conflict field by field, the administrator picks a source, the provenance/conflict resolution is updated, and affected Proposals are rebuilt.

## Proposal Query and Rebuild

Proposal filtering by resource uses the Proposal API:

```http
GET  /api/v1/proposals?resourceKey={resourceKey}
POST /api/v1/versioning/pages/{pageKey}/regenerate
```

Proposal rebuilds rely only on the persisted FunctionContract and CapabilitySemantics. They never guess UI from function names, browser requests, static locales, or arbitrary historical page results.

## Resource Boundaries

- Resource CRUD pages are the default high-quality path, but a Resource is not hard-bound to a database table, and no generic database CRUD API is exposed.
- Business data is executed only through registered functions and PublishedPageSpec bindings.
- Operations, Tasks, and Reports may associate with a Resource or exist standalone.
- The Resource Catalog does not generate console menus; menus are generated only from PublishedPageSpec.

## API Boundaries

- The Resource API reads persisted semantic facts and does not treat in-memory Registry aggregations as the source of truth.
- The Resource API edits only CapabilitySemantics; it accepts no page columns, action placement, navigation, form presentation, or mappings.
- The Resource API does not modify business objects directly; business data is executed only through PublishedPageSpec bindings.
