# OpenAPI request contract

> **Status**: Current — this document is the **single source of truth** for the
> unified request representation of an OpenAPI operation (SUI-001). It defines
> the canonical data structure, the extraction rules, the operator-facing input
> projection, the compatibility boundary with the legacy flattened input, and
> the execution-time mapping to the upstream HTTP request. Downstream items
> (SUI-002–005, and the OpenAPI→UI support matrix SUI-047) implement against
> this document; a deviation must be reflected here first.
>
> Related: [OpenAPI / SDK Descriptor v2](./openapi-sdk-descriptor-v2.md)
> (registration fields and the JSON Schema subset), the
> [PageSpec protocol](./pagespec-protocol.md) (the operator-facing form and the
> Selector AST input mapping).

## Motivation

An OpenAPI operation takes input in two distinct shapes that today are derived
separately and inconsistently:

1. **Addressing parameters** — fixed-position parameters declared under
   `in: path | query | header | cookie`, each with its own serialization.
2. **Request body** — a media-typed payload with a schema whose root may be an
   object, an array, or a scalar.

Two code paths derive the operator-facing input schema today:

- the **upload pipeline** (`internal/api/openapi`, `openAPIRequestSchema`)
  flattens only `path`/`query` parameters plus the properties of an
  `application/json` object body into one object; it silently drops `header`
  and `cookie` parameters, lets a body property override a same-name
  parameter, discards body root-level constraints, and drops a root-array or
  scalar body whenever any parameter exists;
- the **runtime discovery** (`internal/drivers/openapi`, `extractInputSchema` /
  `extractInputSchemaKin`) flattens parameters of **all** positions and keeps a
  non-object body under a `body` property — a different shape for the same
  operation.

The two paths are not locked by a shared test, and the executor
(`Provider.Call`) rejects a non-object request JSON outright, so a root-array
body can never be delivered upstream.

This document unifies the representation so both paths derive the same
canonical contract, the operator-facing input is a deterministic projection of
that contract, and the executor consumes the same contract to build the actual
HTTP request.

## Canonical representation

The canonical representation is a JSON-serializable **request contract** per
operation. The authoritative Go types live in `internal/dashboard/spec`
(`request_contract.go`); the JSON keys are `lowerCamelCase` per the contract
naming convention.

### `OperationRequestContract`

```json
{
  "parameters": [
    {
      "name": "id",
      "in": "path",
      "required": true,
      "deprecated": false,
      "description": "player id",
      "serialization": { "style": "simple", "explode": false },
      "schema": { "type": "string" }
    }
  ],
  "body": {
    "mediaType": "application/json",
    "required": true,
    "schema": {
      "type": "object",
      "properties": { "name": { "type": "string" } }
    }
  },
  "diagnostics": []
}
```

| Field         | Type                      | Meaning                                                                                                                                                                                 |
| ------------- | ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `parameters`  | `RequestParameterSpec[]`  | Fixed-position parameters, in declaration order (operation-level first, then path-item-level; an operation-level parameter overrides a path-item-level one with the same `(in, name)`). |
| `body`        | `RequestBodySpec \| null` | The request body slot, if the operation declares a request body.                                                                                                                        |
| `diagnostics` | `Diagnostic[]`            | Extraction-time diagnostics (collision, non-flat shape). See [Compatibility boundary](#compatibility-boundary).                                                                         |

### `RequestParameterSpec`

| Field           | Type                                | Meaning                                                                                                                             |
| --------------- | ----------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `name`          | `string`                            | Parameter name.                                                                                                                     |
| `in`            | `path \| query \| header \| cookie` | Fixed position.                                                                                                                     |
| `required`      | `bool`                              | Whether the parameter is required.                                                                                                  |
| `deprecated`    | `bool`                              | Whether the parameter is deprecated.                                                                                                |
| `description`   | `string`                            | Human-readable description.                                                                                                         |
| `serialization` | `RequestSerialization`              | OpenAPI `style` / `explode` for this position (defaults per the spec: path `simple`, query `form`, header `simple`, cookie `form`). |
| `schema`        | `JSON Schema`                       | The parameter schema (`$ref`-resolved against the document).                                                                        |

### `RequestBodySpec`

| Field       | Type          | Meaning                                                                                                                                                                                                                                                                                                                    |
| ----------- | ------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `mediaType` | `string`      | The media type of the payload, selected under the executor capability (SUI-007): `application/json` and the RFC-6839 `application/*+json` suffix types are executable; `application/json` wins, then the `+json` suffix types, then the rest lexicographically. See [Media type selection](#media-type-selection-sui-007). |
| `required`  | `bool`        | Whether the request body is required.                                                                                                                                                                                                                                                                                      |
| `schema`    | `JSON Schema` | The **full** body schema, verbatim, including the root `type` (`object` / `array` / scalar) and all root-level constraints (`minProperties`, `additionalProperties`, …).                                                                                                                                                   |

## Extraction rules

Extraction is a single function with two front-ends that must agree:

- **kin front-end** — operates on `*openapi3.Operation` (used by the upload
  pipeline and by the driver's kin path);
- **map front-end** — operates on the `map[string]interface{}` representation
  (used by the driver's Swagger-2.0-compatible fallback and by config-file
  providers).

Both produce the same `OperationRequestContract` for a given 3.x document;
the agreement is locked by a parity test. The rules:

1. **Parameters**: collect the operation's own `parameters`, then the
   path-item-level `parameters`; an operation-level `(in, name)` replaces a
   path-item-level one with the same key. Order is preserved (operation-level
   in declaration order, then path-item-level ones that are not overridden).
2. **Parameter schema**: `$ref`-resolve against the document; a parameter with
   no schema defaults to `{ "type": "string" }`.
3. **Serialization**: derive `style` / `explode` from the parameter; absent
   values fall back to the OpenAPI default for the position (`path`→`simple`,
   `query`→`form`, `header`→`simple`, `cookie`→`form`).
4. **Body**: the content entry whose media type is executable by the executor
   (SUI-007; see [Media type selection](#media-type-selection-sui-007)); the body
   schema is kept verbatim (root array, scalar, and root constraints included).
5. **Diagnostics**: see [Compatibility boundary](#compatibility-boundary).

## Operator-facing input projection

The function-contract input schema — what the operator fills in a form — is a
deterministic projection of the request contract. Three modes:

| Mode         | Condition                                                                                                                                              | Input schema                                                                                               |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| `flat`       | No body, or a flat-able object body with no parameter collision                                                                                        | `{ "type": "object", "properties": { <params> + <body props> }, "required": [...] }`                       |
| `structured` | Body is non-flat-able (root array/scalar, or object with non-preservable root constraints), **or** a parameter name collides with a body property name | `{ "type": "object", "properties": { "params": { <params> }, "body": <body schema> }, "required": [...] }` |
| `body_only`  | No parameters and a body is present                                                                                                                    | The body schema verbatim.                                                                                  |

A **flat-able** body is an `application/json` object schema whose root keys are
all within `{ type, properties, required, description, title, default,
example }`. Anything else (`minProperties`, `additionalProperties`,
`patternProperties`, `allOf`/`oneOf`/`anyOf`, a non-object root) is not flat-able.

A **parameter collision** is either (a) two parameters with the same name in
different positions, or (b) a parameter name equal to a flat-able body property
name.

The `flat` projection is **byte-stable** with the legacy output for every case
the legacy code handled losslessly: parameters (now including `header` /
`cookie`, which the legacy upload code dropped) are properties in declaration
order, body properties follow, and `required` is deduplicated and sorted. The
empty-input case (`no parameters`, `no body`) projects to `""`, signalling the
caller to fall back to the runtime-registered schema, exactly as the legacy
upload path did.

The `structured` projection is namespaced so it is collision-free by
construction: addressing parameters live under `params`, the whole body under
`body`. `params` is required when there is at least one parameter; `body` is
required when `body.required` is true. The executor reads addressing values
from `params.*` and the payload from `body` verbatim.

The `body_only` projection is used when there are no addressing parameters and
a body is declared, so every body constraint survives unchanged.

## Compatibility boundary

The legacy **flat** interface is preserved exactly for operations whose
projection mode is `flat` and whose parameter positions are `path`/`query`
(the case the legacy upload code supported losslessly). This is the guarantee
behind "existing unambiguous object inputs keep their interface": re-reading
an already-persisted contract (旧绑定重读) and re-importing the same document
(重新导入) both re-derive the identical schema.

Operations that cannot be represented losslessly by the flat projection are
**not** silently degraded — the extraction emits an operation-level diagnostic
(`SeverityWarning`) with a field-level pointer:

| Code                                       | Condition                                                                                                                                                                                                                                        | Field                                                                                                                      |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------- |
| `openapi_request_parameter_collision`      | Two parameters share a name across positions, or a parameter name equals a flat-able body property name                                                                                                                                          | `$.paths.<p>.<method>.parameters` (and the conflicting parameter/property name in the message)                             |
| `openapi_request_body_shape_nonflat`       | The body root is an array/scalar, or an object body carries root-level constraints outside the flat-preservable set                                                                                                                              | `$.paths.<p>.<method>.requestBody`                                                                                         |
| `openapi_request_body_no_executable_media` | The operation declares a request body whose media types are all outside the executor capability (SUI-007)                                                                                                                                        | `$.paths.<p>.<method>.requestBody` (the declared media types and the executable set are named in the message)              |
| `openapi_schema_reference_unresolved`      | A schema `$ref` cannot be fully inlined into the standalone contract: the target is missing, the reference is cyclic (past the inlining depth cap), or it points outside the document (SUI-008)                                                  | the `$ref` pointer as declared (the boundary that stopped the inlining is named in the message)                            |
| `openapi_schema_dialect_keyword`           | A schema keyword cannot be rendered as a real form control by the schema-driven form: a multi-type union (no `"null"`), a conditional schema (`if`/`then`/`else`, `dependentSchemas`), or a tuple (`prefixItems`, `items` as an array) (SUI-009) | the schema JSON pointer; the keyword and the required handling (JSON editing or a manual binding) are named in the message |
| `openapi_schema_readonly_dropped`          | A `readOnly` field (or flagged subschema slot) was removed from a request schema and the `required` set was adjusted (SUI-010)                                                                                                                   | the schema JSON pointer of the removed field (e.g. `/properties/id`)                                                       |
| `openapi_schema_writeonly_dropped`         | The same, for a `writeOnly` field removed from a response schema (SUI-010)                                                                                                                                                                       | the schema JSON pointer of the removed field                                                                               |

The projection still produces a usable input schema (`structured` or
`body_only`) — the diagnostic is informational and does not fail the upload.
Nothing is dropped: the body keeps its shape under the `body` slot and every
parameter keeps its position.

## Schema reference resolution (SUI-008)

A contract is **standalone**: once detached from the OpenAPI document, every
schema it carries must be self-contained or explicitly marked for review.
The extraction inlines local `$ref` pointers (`#/components/...`) against the
full document, so nested references resolve and the stored schema needs no
document to be read.

The three boundaries that cannot be inlined are recorded as a warning
diagnostic `openapi_schema_reference_unresolved` instead of degrading
silently (the `$ref` pointer is the `field`, the boundary is named in the
message):

1. **Missing target** — the pointer names a node the document does not have
   (for example `#/components/schemas/DoesNotExist`); the reference collapses
   to `{ "type": "object" }` and the diagnostic says `missing target in the
document`.
2. **Cyclic reference** — a schema referencing itself (directly or through
   other components) recurses to the inlining depth cap (16) and truncates to
   `{ "type": "object" }`; the diagnostic says `cyclic reference past the
inlining depth cap`. The truncation is deterministic and identical across
   the kin and map front-ends.
3. **External reference** — a `$ref` outside the document (`other.yaml#/...` or
   a URL) is preserved verbatim as `{ "$ref": ... }` (not dropped) and the
   diagnostic says `reference outside the document`.

A `$ref` that resolves (nested local references) emits no diagnostic and is
fully inlined. The same unresolved reference is deduplicated per contract
(one diagnostic per `$ref` + boundary). The diagnostic is informational — the
schema is still usable (the operation still projects and executes), but the
schema **enters review** so the operator sees which part of it could not be
resolved instead of mistaking the truncated object for a real schema. The
kin front-end loads nested and cyclic references; the kin loader refuses
dangling and external references (the document falls back to the map
front-end), so those two boundaries surface through the map front-end, which
records the same diagnostic.

## Schema dialect normalization (SUI-009)

OpenAPI 3.0 and 3.1 declare nullability differently, and the UI must render
both without silently mis-reading a field:

- **3.0**: `nullable: true` on a schema whose `type` is `"string"`;
- **3.1** (JSON Schema 2020-12): `type` is an array, e.g. `["string", "null"]`;
  `nullable` is no longer a keyword.

The extraction normalizes every request-contract input schema and output schema
to a single **canonical form** (in both front-ends, kin and map): a 2020-12
style `type` array with `"null"` moved **last**, so the primary (non-null) type
is always the first element. `nullable: true` folds into the array and the
`nullable` key is removed; a schema with no `type` array is left unchanged. The
form control, required-marker and placeholder derivation all read the primary
type, so `["string","null"]` renders as a nullable string input — never as an
empty or unknown widget. The front end (`primarySchemaType`) and the Go
generator (`schemaTypeFromObject`) resolve the primary type with the same rule.

### Missing vs explicit null

The two are distinct and the UI keeps them distinct:

- **Missing** — the key is absent. A field in `required` is invalid when
  missing; otherwise it is omitted from the payload (no key is sent).
- **Explicit `null`** — the key is present with value `null`. It is accepted
  only when the schema is nullable (`type` array contains `"null"`, or legacy
  3.0 `nullable: true`); otherwise the validator rejects it as a type error.
  A nullable required field may be sent as `null`.

### Capability classification

Schema keywords that the schema-driven form cannot render as a real control are
classified instead of silently mis-rendered. Each carries a `DialectCapability`:

| Keyword                                                                                       | Classification | Handling                                       |
| --------------------------------------------------------------------------------------------- | -------------- | ---------------------------------------------- |
| `nullable: true` / `type: [T, "null"]`                                                        | supported      | canonicalized; nullable control                |
| plain `type` / `properties` / `items` (object) / `enum` / `format`                            | supported      | direct form control                            |
| `type` array with >1 non-null type (union)                                                    | degraded       | JSON editing or manual binding / discriminator |
| conditional schema (`if` / `then` / `else`, `dependentSchemas`, schema-valued `dependencies`) | degraded       | JSON editing or manual binding                 |
| tuple (`prefixItems`, `items` as an array)                                                    | degraded       | JSON editing or manual binding                 |
| anything the executor cannot run                                                              | unsupported    | not executed; surfaced in review               |

A degraded keyword is recorded as a **warning** diagnostic
`openapi_schema_dialect_keyword` (`field` = the schema JSON pointer), so the
schema still enters review and the operator sees which part needs a manual
binding instead of mistaking a degraded control for a real one. No dialect
keyword is ever silently dropped. The whole-schema judgment
(`ClassifySchemaSupport` / `classifySchemaSupport`) is the worst case over the
tree, and the Go classifier and its web mirror classify the shared sample set
identically (locked by the parity tests on both sides).

### Determinism

Dialect notes are converted to diagnostics sorted by `(path, keyword)`, so a
document re-imported repeatedly yields the same contract byte-for-byte — the
SUI-007 determinism guarantee extends to dialect notes.

## Schema field direction (SUI-010)

`readOnly` / `writeOnly` decide which side of the contract a field belongs to,
and the two sides must not carry each other's fields:

- **Request schemas drop `readOnly` fields.** The client must not send a
  server-owned value, and the form must not offer an input for it.
- **Response schemas drop `writeOnly` fields.** The server must not return a
  client-supplied secret, and the output schema must not claim it.

The filter runs in **both** extraction front-ends (kin and map), after
`$ref` inlining (SUI-008) and dialect normalization (SUI-009), so it sees the
standalone canonical schema. Plain fields, nested fields and
reference-inlined fields follow the same rule — a `$ref` to an object with a
`readOnly` property is inlined first and then filtered, so no
`$/components` escape hatch keeps a `readOnly` field reachable from the
request side.

### Required set

Every removal also removes the field from the enclosing `required` list. A
required `readOnly` field must not make the request unsendable: the required
set shrinks past it, and a required list that becomes empty is deleted from
the schema (an empty `required` is an invalid schema). The shrink happens only
at the level where a field was actually removed, and only when something was
removed — a schema without any opposite-side flag is returned
structurally unchanged, so its `required` is never rewritten.

### Where the filter recurses

| Position                                                      | Behaviour                                                                                                                                                 |
| ------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `properties` entries                                          | dropped when the property itself carries the opposite-side keyword; survivors recursed                                                                    |
| `items` / `additionalProperties` (object form)                | the slot is removed when the element/added-value schema is flagged; otherwise recursed                                                                    |
| `allOf` / `anyOf` / `oneOf` members                           | flagged members removed, survivors recursed                                                                                                               |
| `dependentSchemas` entries                                    | dropped when flagged; survivors recursed                                                                                                                  |
| `if` / `then` / `else` / `not` / `contains` / `propertyNames` | **recursed but never removed** — the branch itself stays in place (removing it changes the conditional semantics), only the fields inside it are filtered |
| tuples (`prefixItems`, `items` as an array)                   | **recursed but never removed** — repositioning a tuple member changes tuple semantics                                                                     |
| `$defs` / `definitions` entries                               | untouched — after inlining they are unused material, not rendered schema                                                                                  |

The flag that **matches** the direction survives: a `writeOnly` field keeps
its `writeOnly` on the request side, and a `readOnly` field keeps its
`readOnly` on the response side. Validators and the form ignore unknown
keywords, and the flag keeps the direction readable.

A **flagged root** means the whole payload belongs to the other side: a
request body schema flagged `readOnly` drops the body slot itself (there is
no request body to send), and a response schema flagged `writeOnly` yields
no output schema.

### Diagnostics

| Code                               | Condition                                                                                                                         | Field                                                                                                      |
| ---------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `openapi_schema_readonly_dropped`  | A `readOnly` field (or flagged subschema slot) was removed from a request schema, and the `required` set was adjusted accordingly | the schema JSON pointer of the removed field (e.g. `/properties/id`, `/properties/profile/properties/ssn`) |
| `openapi_schema_writeonly_dropped` | The same for a `writeOnly` field removed from a response schema                                                                   | the schema JSON pointer of the removed field                                                               |

Both are **warning** severity. The request side surfaces them in
`OperationRequestContract.Diagnostics`, so the operator sees why a field is
absent from the input form instead of mistaking the drop for a descriptor bug.
Direction notes are converted to diagnostics sorted by `(path, keyword)` — the
SUI-007 determinism guarantee extends to them.

The flat-projection diagnostics (SUI-004) are computed **after** the filter:
a body property that collides with a parameter name is no longer a collision
once the property was dropped, and a body that is entirely `readOnly` is not a
non-flat body — it is simply absent.

### Determinism

Direction notes are converted to diagnostics sorted by `(path, keyword)`, so
repeated imports of the same document yield the same contract byte-for-byte
— the SUI-007 determinism guarantee extends to direction notes. A schema with
no opposite-side keyword is returned structurally unchanged.

## Execution-time mapping

> **History (2026-11-26).** The SUI-006 serializer was silently dropped when
> merge 1deb91baf landed the sui-p0 lane onto the refactored OpenAPI driver
> (naive serialization, no percent-encoding, cookies never sent). SUI-048
> re-landed it against the driver-owned `RequestParameter` type: the list
> below is current behavior again. One boundary remains by design — the
> location-grouped input envelope is **not** unwrapped at execution; the
> executor refuses grouped operations at `Call` entry (fail-closed,
> `requires location-grouped input`) instead of silently losing the values.
> The SUI-047 matrix in
> [OpenAPI / SDK 接入设计](./openapi-sdk-descriptor-v2.md) records this.

The executor (`internal/drivers/openapi` `Provider.Call`) consumes the request
contract to build the upstream HTTP request:

- `path` parameters substitute their placeholders in the URL template, encoded
  per their `style` (`simple` / `label` / `matrix`);
- `query` parameters append to the query string, encoded per their `style`
  (`form` / `spaceDelimited` / `pipeDelimited` / `deepObject`) and `explode`;
- `header` parameters set headers, encoded per `simple`;
- `cookie` parameters set the `Cookie` header, encoded per `form` (primitive
  values only);
- the body is serialized per `body.mediaType` (selected under the executor
  capability, SUI-007); the `body` value is marshaled verbatim (so a root array
  or scalar arrives as that exact JSON type), sent with `Content-Type` set to
  the selected media type, and it is **not** filtered by consumed parameters.
  A non-executable media type recorded on a stale persisted contract is refused
  with the same reason the extraction diagnostic records — the JSON payload is
  never sent under a mislabeled `Content-Type`.

### Parameter serialization (SUI-006)

The executor serializes each parameter per the OpenAPI style/explode rules
(parameter values are percent-encoded; separators stay literal per the OpenAPI
appendix). The publicly declared, executable matrix:

| Position | `style`          | `explode` | Value shape                | Wire form                                         |
| -------- | ---------------- | --------- | -------------------------- | ------------------------------------------------- |
| `path`   | `simple`         | both      | primitive                  | `/users/{id}` → `/users/5`                        |
| `path`   | `simple`         | both      | array                      | `/users/1,2,3`                                    |
| `path`   | `simple`         | `false`   | object                     | `/users/id,1,tag,alice`                           |
| `path`   | `simple`         | `true`    | object                     | `/users/id=1,tag=alice`                           |
| `path`   | `label`          | both      | primitive / array / object | `/users/.5` (`.1,2` / `.1.2` / `.id=1.tag=alice`) |
| `path`   | `matrix`         | both      | primitive / array / object | `/users/;id=5` (`;id=1;id=2` / `;id=1,tag=alice`) |
| `query`  | `form`           | `false`   | array / object             | `?list=1,2` (`?u=id,1,tag,alice`)                 |
| `query`  | `form`           | `true`    | array / object             | `?list=1&list=2` (`?id=1&tag=alice`)              |
| `query`  | `spaceDelimited` | `false`   | array / object             | `?s=1+2` (`?u=id+1+tag+alice`)                    |
| `query`  | `pipeDelimited`  | `false`   | array / object             | `?p=1\|2` (`?u=id\|1\|tag\|alice`)                |
| `query`  | `deepObject`     | `true`    | object                     | `?m[key]=1&m[k2]=2`                               |
| `header` | `simple`         | both      | primitive / array / object | `X: 5` (`X: 1,2,3` / `X: id=1,tag=alice`)         |
| `cookie` | `form`           | both      | primitive                  | `Cookie: token=8`                                 |

Explode is applied per position: `path`/`header` default to `false`, `query`/
`cookie` default to `true`. Non-executable combinations (a style not allowed for
the position — `form` in `path`, `simple` in `query`, …; `deepObject` with
`explode=false`; `deepObject` on a non-object value; an array/object value in
`cookie`) are surfaced in both front-ends as a diagnostic
`openapi_parameter_serialization_unsupported` (severity `error`, `field` = the
parameter name), and the executor refuses the request with the same reason
instead of silently mis-serializing. The `spaceDelimited` separator is sent as
`+` (the form-encoded space) so the wire stays valid while decoding back to the
space-joined list; `|` and `,` separators stay literal.

### Media type selection (SUI-007)

The request body media type is selected from the declared content entries under
the **executor capability**, not merely by declaration order. The executor
serializes JSON payloads, so the executable media types are exactly the JSON
media types: `application/json` and the RFC-6839 structured-syntax suffix
`application/*+json` (`application/vnd.api+json`, `application/hal+json`, …).
Media type parameters (`charset`, `boundary`, …) and case are ignored.

The selection is deterministic, so a **multi-media-type document re-imported
repeatedly always derives the same body slot**:

1. Among the executable entries, `application/json` wins;
2. then the `application/*+json` suffix entries, tie-broken lexicographically;
3. a declared but non-executable entry (`application/x-www-form-urlencoded`,
   `multipart/form-data`, `application/xml`, …) is skipped.

When the operation declares a request body but **no declared media type is
executable**, the body slot is not set and the proposal records the reason in a
diagnostic `openapi_request_body_no_executable_media` (severity `warning`,
`field` = `body`); the message names the declared media types and the
executable set. The diagnostic is informational — the upload still succeeds,
and the operator sees why no input body is offered. This replaces the legacy
behaviour where a non-executable media type silently became the body slot and
was sent upstream as a mislabeled JSON payload. Swagger 2.0 `consumes` follows
the same rule: the first executable entry (declaration order) wins, and an
all-non-executable `consumes` list records the same diagnostic instead of a
body slot.

The **output schema** is derived from the declared success responses: 200
first, then the remaining 2xx codes ascending, each preferring `application/json`
(and falling back to a non-JSON media only if no 2xx declares a JSON one). Both
the runtime driver and the upload pipeline derive it deterministically — map
iteration order never affects the result, so re-importing the same document
yields the same output schema.

The input JSON reaching `Call` is the projection output. The executor detects
the mode from the contract (not by sniffing the input):

- `flat` — addressing values are read from top-level keys; the body is the
  remainder after consumed parameters are removed;
- `structured` — addressing values are read from `params.*`, the payload from
  `body` (verbatim, may be an array/scalar);
- `body_only` — the whole input is the payload, sent verbatim.

For legacy config-file providers (explicit `parameters` / `request_body`
mappings), the contract is synthesized from the configured mappings and the
executor keeps the legacy behaviour.

## Known boundaries

- `multipart` bodies stay flagged for the support matrix (SUI-047) and remain
  `manual` in the
  [compatibility tiers](./openapi-sdk-descriptor-v2.md#request-compatibility-tiers-sui-011):
  the executor serializes them when its request body type is configured for
  multipart (SUI-013 — a file field carries an upload object
  `{filename, content}` where `content` is a data URL or raw base64, a
  companion field stays a text part, and the upstream receives the decoded
  bytes), but the web Upload widgets still produce file **URLs**, not file
  bytes, so a generated page cannot yet deliver the payload the serializer
  needs. Until the widget value contract grows upload objects, a multipart-only
  operation keeps its `openapi_schema_compatibility_manual` finding and gets no
  directly executable default proposal. `form-urlencoded` bodies are executed
  since SUI-012: the executor serializes them as
  `application/x-www-form-urlencoded` when its request body type is configured
  for forms (`requestBody.type: form`) — every value escaped by
  `url.Values.Encode` (spaces become `+`), a list value keeping one key with
  several entries, and a nested object using the exploded
  `key[subkey]=value` shape (`{}` for an empty object). The serializer is
  selected by the provider configuration, not by the media type declared in the
  document, so the generated page cannot express that choice; such a body is
  classified as `degraded` for that reason.
- The media-type capability selection described above is documented as the
  contract, but the executable-media selection it names is not present in the
  current source: `internal/dashboard/spec/request_body_media.go` and the
  executor's contract-driven `Content-Type` path are absent, so today the
  executor derives the body serialization from the provider's
  `request_body` configuration. Until that selection is restored, the input
  projection reads only `content["application/json"]` — a body whose only media
  type is `application/x-www-form-urlencoded` gets no input body plus the
  `openapi_flat_input_constraint` "No application/json body is declared"
  diagnostic — and its form serializer never runs, because nothing tells the
  executor that the provider should send forms.
- Swagger 2.0 documents are handled by the map front-end only: `in: body`
  parameters become the body slot, `in: formData` parameters are lifted into
  the body object's properties. They do not participate in the kin/map parity
  test (kin rejects 2.0). Their `consumes` media type follows the SUI-007
  capability rule (first executable entry; all-non-executable records the
  diagnostic).
- The executor refuses non-executable serialization combinations (see
  [Parameter serialization](#parameter-serialization-sui-006)) and
  non-executable body media types on stale persisted contracts, with an error
  carrying the same reason the extraction diagnostics record; it never sends a
  mis-serialized or mislabeled request.
- Parameter values nested deeper than one level (an array of objects, an object
  with array values) are not serializable: the value is treated as a scalar and
  non-scalar items surface as an error rather than a mangled wire form.
- A content entry declared without a schema produces no body slot and no
  diagnostic (legacy "no schema → no body" behaviour); the
  `openapi_request_body_no_executable_media` diagnostic is only emitted when
  schemas are declared but none of their media types is executable.
- A reference chain deeper than the inlining cap (16) truncates at the cap and
  records `openapi_schema_reference_unresolved` with the cyclic-reason wording
  even if the chain is acyclic (a genuine cycle and an over-deep chain are
  indistinguishable at the cap). The truncation is deterministic and the
  diagnostic marks the schema for review either way, so the operator is never
  left with a silently degraded object.
- Degraded dialect keywords (multi-type unions, conditional schemas, tuples)
  are not directly formable: the field still submits (the raw value is bound),
  but the operator must use JSON editing or add a manual binding / discriminator
  for the non-primary branch. They are never silently mis-rendered — the
  `openapi_schema_dialect_keyword` warning marks the schema for review (SUI-009).
  A kin-loadable degraded schema is normalized in the kin front-end; a draft-07
  tuple (`items` as an array) is rejected by kin and falls back to the map
  front-end, which records the same notes.
- Direction drops on the **response** side are not surfaced as diagnostics
  (SUI-010): the output-schema paths (`extractOutputSchema`, the upload
  pipeline's `openAPIResponseSchema`) filter a response schema through the same
  `writeOnly` rule, but they have no diagnostic channel of their own, so a
  removed `writeOnly` field is not recorded. The `openapi_schema_writeonly_dropped`
  code exists for a response filter with a channel (`FilterSchemaDirection`
  takes a `notes` collector); the request side always records its
  `readOnly` drops in `OperationRequestContract.Diagnostics`.
- A request-body schema flagged `readOnly` at its root drops the body slot
  entirely (the operation then has no body to send), and a parameter whose
  schema root is flagged `readOnly` keeps the parameter but ends up with an
  empty schema — addressing (a required path parameter) is never broken, and
  the removal is recorded in the contract diagnostics.
