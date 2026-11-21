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
    "schema": { "type": "object", "properties": { "name": { "type": "string" } } }
  },
  "diagnostics": []
}
```

| Field | Type | Meaning |
|---|---|---|
| `parameters` | `RequestParameterSpec[]` | Fixed-position parameters, in declaration order (operation-level first, then path-item-level; an operation-level parameter overrides a path-item-level one with the same `(in, name)`). |
| `body` | `RequestBodySpec \| null` | The request body slot, if the operation declares a request body. |
| `diagnostics` | `Diagnostic[]` | Extraction-time diagnostics (collision, non-flat shape). See [Compatibility boundary](#compatibility-boundary). |

### `RequestParameterSpec`

| Field | Type | Meaning |
|---|---|---|
| `name` | `string` | Parameter name. |
| `in` | `path \| query \| header \| cookie` | Fixed position. |
| `required` | `bool` | Whether the parameter is required. |
| `deprecated` | `bool` | Whether the parameter is deprecated. |
| `description` | `string` | Human-readable description. |
| `serialization` | `RequestSerialization` | OpenAPI `style` / `explode` for this position (defaults per the spec: path `simple`, query `form`, header `simple`, cookie `form`). |
| `schema` | `JSON Schema` | The parameter schema (`$ref`-resolved against the document). |

### `RequestBodySpec`

| Field | Type | Meaning |
|---|---|---|
| `mediaType` | `string` | The media type of the payload (the first `application/json` entry when present, otherwise the first content entry). |
| `required` | `bool` | Whether the request body is required. |
| `schema` | `JSON Schema` | The **full** body schema, verbatim, including the root `type` (`object` / `array` / scalar) and all root-level constraints (`minProperties`, `additionalProperties`, …). |

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
4. **Body**: the first content entry, preferring `application/json`; the body
   schema is kept verbatim (root array, scalar, and root constraints included).
5. **Diagnostics**: see [Compatibility boundary](#compatibility-boundary).

## Operator-facing input projection

The function-contract input schema — what the operator fills in a form — is a
deterministic projection of the request contract. Three modes:

| Mode | Condition | Input schema |
|---|---|---|
| `flat` | No body, or a flat-able object body with no parameter collision | `{ "type": "object", "properties": { <params> + <body props> }, "required": [...] }` |
| `structured` | Body is non-flat-able (root array/scalar, or object with non-preservable root constraints), **or** a parameter name collides with a body property name | `{ "type": "object", "properties": { "params": { <params> }, "body": <body schema> }, "required": [...] }` |
| `body_only` | No parameters and a body is present | The body schema verbatim. |

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

| Code | Condition | Field |
|---|---|---|
| `openapi_request_parameter_collision` | Two parameters share a name across positions, or a parameter name equals a flat-able body property name | `$.paths.<p>.<method>.parameters` (and the conflicting parameter/property name in the message) |
| `openapi_request_body_shape_nonflat` | The body root is an array/scalar, or an object body carries root-level constraints outside the flat-preservable set | `$.paths.<p>.<method>.requestBody` |

The projection still produces a usable input schema (`structured` or
`body_only`) — the diagnostic is informational and does not fail the upload.
Nothing is dropped: the body keeps its shape under the `body` slot and every
parameter keeps its position.

## Execution-time mapping

The executor (`internal/drivers/openapi` `Provider.Call`) consumes the request
contract to build the upstream HTTP request:

- `path` parameters substitute their placeholders in the URL template;
- `query` parameters append to the query string;
- `header` parameters set headers;
- `cookie` parameters set the `Cookie` header (executed by SUI-006; the
  executor today sets `path`/`query`/`header` and carries `cookie` on the
  contract without sending it);
- the body is serialized per `body.mediaType`; for `application/json` the
  `body` value is marshaled verbatim (so a root array or scalar arrives as that
  exact JSON type), and it is **not** filtered by consumed parameters.

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

- `form-urlencoded` and `multipart` bodies are declared but not yet executed
  (SUI-012 / SUI-013); they are carried by `body.mediaType` and flagged for the
  support matrix (SUI-047).
- Swagger 2.0 documents are handled by the map front-end only: `in: body`
  parameters become the body slot, `in: formData` parameters are lifted into
  the body object's properties. They do not participate in the kin/map parity
  test (kin rejects 2.0).
- Deep-object, matrix, and label serializations for `query`/`path`, and cookie
  parameters as a whole, are captured in the contract but not yet executed by
  the executor (SUI-006); unsupported combinations produce a diagnostic rather
  than a wrong request.
