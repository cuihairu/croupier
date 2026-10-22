# Function API

## Common Types

```go
type JSONValue = json.RawMessage
type JSONSchema = json.RawMessage
type FormPresentationSpec = json.RawMessage
type OpenAPIOperation = json.RawMessage
```

Notes:

- `JSONValue` only represents the business payload or a function return value; its structure must be constrained by the function's `inputSchema` / `outputSchema`.
- `JSONSchema` only represents a JSON Schema / OpenAPI Schema.
- `FormPresentationSpec` represents the controlled presentation configuration of a JSON Schema form; it must not carry page layout, menus, or arbitrary component props.
- `OpenAPIOperation` is for contract viewing only; it is not used to generate runtime console pages directly.
- `json.RawMessage` is only the JSON carrier type at the HTTP boundary; the server must complete structural validation before saving or executing.

### 1. "List functions"

1. route definition

- Url: /api/v1/functions
- Method: GET
- Request: `FunctionsListRequest`
- Response: `FunctionsListResponse`

2. request definition

```go
type FunctionsListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	GameId string `form:"gameId,optional"`
	Category string `form:"category,optional"`
	Status int `form:"status,optional"`
}
```

3. response definition

```go
type FunctionsListResponse struct {
	Items []Function `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "Get function details"

1. route definition

- Url: /api/v1/functions/:id
- Method: GET
- Request: `FunctionDetailRequest`
- Response: `FunctionDetailResponse`

2. request definition

```go
type FunctionDetailRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionDetailResponse struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	GameId string `json:"gameId"`
	Status int `json:"status"`
	Version string `json:"version"`
	Instances int `json:"instances"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Descriptor FunctionDescriptor `json:"descriptor"`
}

type Function struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	GameId string `json:"gameId"`
	Status int `json:"status"`
	Version string `json:"version"`
	Instances int `json:"instances"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type FunctionDescriptor struct {
	Input JSONSchema `json:"input"`
	Output JSONSchema `json:"output"`
	Schema JSONSchema `json:"schema"`
}
```

### 3. "Delete a function"

1. route definition

- Url: /api/v1/functions/:id
- Method: DELETE
- Request: `FunctionActionRequest`
- Response: `-`

2. request definition

```go
type FunctionActionRequest struct {
	ID string `path:"id"`
}
```

3. response definition

### 4. "Copy a function"

1. route definition

- Url: /api/v1/functions/:id/copy
- Method: POST
- Request: `FunctionCopyRequest`
- Response: `FunctionCopyResponse`

2. request definition

```go
type FunctionCopyRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionCopyResponse struct {
	FunctionId string `json:"functionId"`
	NewId string `json:"newId"`
}
```

### 5. "Disable a function"

1. route definition

- Url: /api/v1/functions/:id/disable
- Method: POST
- Request: `FunctionActionRequest`
- Response: `-`

2. request definition

```go
type FunctionActionRequest struct {
	ID string `path:"id"`
}
```

3. response definition

### 6. "Enable a function"

1. route definition

- Url: /api/v1/functions/:id/enable
- Method: POST
- Request: `FunctionActionRequest`
- Response: `-`

2. request definition

```go
type FunctionActionRequest struct {
	ID string `path:"id"`
}
```

3. response definition

### 7. "Get function instances"

1. route definition

- Url: /api/v1/functions/:id/instances
- Method: GET
- Request: `FunctionInstancesRequest`
- Response: `FunctionInstancesResponse`

2. request definition

```go
type FunctionInstancesRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionInstancesResponse struct {
	Items []FunctionInstance `json:"items"`
}
```

**Cross-instance aggregation** (cluster mode): this endpoint and `GET /api/v1/functions/instances`
both treat the shared ownership table (`cluster_agent_owners`) as the online universe — agents held
by peer server instances count too, with details read from the shared `agent_sessions` snapshot
table. Entries owned by a peer instance carry an `ownerInstance` field (covering two cases: those
absent from the local registry and recovered via the snapshot table, and those present locally but
owned by a peer in the DB snapshot — the latter backfilled by the 30s-periodic
`refreshRemoteSnapshots`, not directly connected to this instance); the field is absent for agents
this instance owns or connects to directly. When the ownership table is unreachable, it silently
falls back to this instance's view (everything rendered as local). Single-instance deployments
(`cluster.enabled=false`) behave unchanged (purely local).

### 8. "Invoke a function"

1. route definition

- Url: /api/v1/functions/:id/invoke
- Method: POST
- Request: `FunctionInvokeRequest`
- Response: `FunctionInvokeResponse`

2. request definition

```go
type FunctionInvokeRequest struct {
	ID string `path:"id"`
	Params JSONValue `json:"params,optional"`
	Payload JSONValue `json:"payload,optional"`
	GameID string `json:"gameId,optional"` // compatibility field; the effective scope is the X-Game-ID/X-Env headers
	Env string `json:"env,optional"`     // compatibility field; the effective scope is the X-Game-ID/X-Env headers
	Mode string `json:"mode,optional"`
	Route string `json:"route,optional"`
	TargetServiceID string `json:"targetServiceId,optional"`
	HashKey string `json:"hashKey,optional"`
}
```

3. response definition

```go
type FunctionInvokeResponse struct {
	TaskId           string      `json:"taskId"`
	Result           JSONValue   `json:"result,omitempty"`
	ApprovalID       string      `json:"approval_id,omitempty"`       // approval request ID (returned when approval is required)
	ApprovalRequired bool        `json:"approval_required,omitempty"` // whether approval is required
	ApprovalWorkflow string      `json:"approval_workflow,omitempty"` // approval workflow type (single_admin/two_person)
}
```

**Notes:**

- When the function policy requires approval (`RequireApproval=true`), the invocation creates an approval request and returns `ApprovalID`
- An invocation requiring approval does not execute immediately; it waits until the approval passes
- `ApprovalWorkflow` indicates the approval workflow type:
  - `single_admin`: a single administrator's approval suffices
  - `two_person`: two-person approval required

**Route parameter validation (behavior change 2026-09-11)**: the following cases return `400 validation_failed` directly before the policy check, **no longer silently falling back to the default route**:

- `route: "targeted"` without `targetServiceId`
- `route: "hash"` without `hashKey`
- `route: "broadcast"` combined with `mode: "async"` (broadcast has no async semantics)
- an invalid `route` value (valid: `lb` (default)/`targeted`/`hash`/`broadcast`)

**Error semantics**:

- `503 service_unavailable`: no available agent (no candidates in either the local or the shared ownership table) or failover exhausted all candidates (same semantics for synchronous and `mode: async`; lb routing retries up to 3 candidate switches, targeted/hash designate a unique target and do not retry) — retryable (self-heals after agents re-register or the ownership table TTL expires); in multi-instance deployments, when a remote candidate is chosen, the call forwards over the mesh to the owner instance, transparent to the caller
- `400 validation_failed`: missing/conflicting route parameters (see above), with `details` carrying the specific fields

### 9. "Get function permissions"

1. route definition

- Url: /api/v1/functions/:id/permissions
- Method: GET
- Request: `FunctionPermissionsRequest`
- Response: `FunctionPermissionsResponse`

2. request definition

```go
type FunctionPermissionsRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionPermissionsResponse struct {
	Items []FunctionPermission `json:"items"`
}
```

### 10. "Update function permissions"

1. route definition

- Url: /api/v1/functions/:id/permissions
- Method: PUT
- Request: `FunctionPermissionsUpdateRequest`
- Response: `FunctionPermissionsResponse`

2. request definition

```go
type FunctionPermissionsUpdateRequest struct {
	ID string `path:"id"`
	Permissions []FunctionPermission `json:"permissions"`
}
```

3. response definition

```go
type FunctionPermissionsResponse struct {
	Items []FunctionPermission `json:"items"`
}
```

### 11. "Publish a function"

1. route definition

- Url: /api/v1/functions/:id/publish
- Method: POST
- Request: `FunctionPublishRequest`
- Response: `FunctionPublishResponse`

2. request definition

```go
type FunctionPublishRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionPublishResponse struct {
	ApprovalId string `json:"approvalId,omitempty"` // when approval is required
	Published bool `json:"published"`
}
```

### 12. "Batch copy functions"

1. route definition

- Url: /api/v1/functions/batch-copy
- Method: POST
- Request: `BatchCopyFunctionsRequest`
- Response: `BatchCopyFunctionsResponse`

2. request definition

```go
type BatchCopyFunctionsRequest struct {
	FunctionIds []string `json:"function_ids"`
}
```

3. response definition

```go
type BatchCopyFunctionsResponse struct {
	Updated int `json:"updated"`
	Failed []string `json:"failed"`
	Copied []string `json:"copied"` // list of newly copied function IDs
}
```

### 17. "Batch delete functions"

1. route definition

- Url: /api/v1/functions/batch-delete
- Method: POST
- Request: `BatchDeleteFunctionsRequest`
- Response: `BatchDeleteFunctionsResponse`

2. request definition

```go
type BatchDeleteFunctionsRequest struct {
	FunctionIds []string `json:"function_ids"`
}
```

3. response definition

```go
type BatchDeleteFunctionsResponse struct {
	Updated int `json:"updated"`
	Failed []string `json:"failed"`
}
```

### 18. "Batch update function status"

1. route definition

- Url: /api/v1/functions/batch-update
- Method: POST
- Request: `BatchUpdateFunctionsRequest`
- Response: `BatchUpdateFunctionsResponse`

2. request definition

```go
type BatchUpdateFunctionsRequest struct {
	FunctionIds []string `json:"function_ids"`
	Enabled bool `json:"enabled"`
}
```

3. response definition

```go
type BatchUpdateFunctionsResponse struct {
	Updated int `json:"updated"`
	Failed []string `json:"failed"`
}
```

### 19. "List function descriptors"

1. route definition

- Url: /api/v1/functions/descriptors
- Method: GET
- Request: `DescriptorsRequest`
- Response: `DescriptorsResponse`

2. request definition

```go
type DescriptorsRequest struct {
	Type string `form:"type,optional"`
	GameId string `form:"gameId,optional"`
}
```

3. response definition

```go
type DescriptorsResponse struct {
	Items []Descriptor `json:"items"`
}
```

### 20. "List pending functions"

1. route definition

- Url: /api/v1/functions/pending
- Method: GET
- Request: `FunctionsPendingRequest`
- Response: `FunctionsPendingResponse`

2. request definition

```go
type FunctionsPendingRequest struct {
}
```

3. response definition

```go
type FunctionsPendingResponse struct {
	Items []PendingFunction `json:"items"`
}
```

### 21. "Batch fetch function OpenAPI"

1. route definition

- Url: /api/v1/functions/\_openapi-batch
- Method: POST
- Request: `BatchGetSpecRequest`
- Response: `map[string]OpenAPIOperation`

2. request definition

```go
type BatchGetSpecRequest struct {
	FunctionIDs []string `json:"function_ids"`
}
```

3. response definition

```go
// key is the function id, value the corresponding OpenAPI Operation; returns null when not found
map[string]OpenAPIOperation
```

### Notes

- This endpoint lets the Dashboard batch-read function OpenAPI instead of requesting one by one.
- The current return value passes through the OpenAPI operation objects stored in the registry verbatim.

### 22. "Get function contract change history"

1. route definition

- Url: /api/v1/functions/:id/versions
- Method: GET
- Request: query `page` / `pageSize`
- Response: `ContractVersionsResult`

2. response definition

```go
type ContractVersionDiffEntry struct {
	Field    string                 `json:"field"`
	From     string                 `json:"from,omitempty"`
	To       string                 `json:"to,omitempty"`
	Change   string                 `json:"change,omitempty"` // e.g. schema_replaced
	Findings []schemadiff.Finding   `json:"findings,omitempty"`
}

type ContractVersionItem struct {
	Seq          int64    `json:"seq"`
	Version      string   `json:"version,omitempty"`
	Source       string   `json:"source,omitempty"`
	SourceDigest string   `json:"sourceDigest,omitempty"`
	ChangeType   string   `json:"changeType"` // created|updated|removed
	Breaking     bool     `json:"breaking"`
	Actor        string   `json:"actor,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	Diff         []ContractVersionDiffEntry `json:"diff,omitempty"`
}

type ContractVersionsResult struct {
	Items []ContractVersionItem `json:"items"`
	Total int64                 `json:"total"`
	Page  int                   `json:"page"`
	Size  int                   `json:"size"`
}
```

### 23. "Get a function contract version snapshot"

1. route definition

- Url: /api/v1/functions/:id/versions/:seq
- Method: GET
- Response: `ContractVersionDetailResult` (`ContractVersionItem` + `snapshot`; snapshot is the post-change FunctionSpec projection; 404 when absent)

### 24. "Diff two function contract versions"

1. route definition

- Url: /api/v1/functions/:id/versions/diff
- Method: GET
- Request: query `from` / `to` (version seq)
- Response: `ContractVersionDiffResult`

```go
type ContractVersionDiffResult struct {
	FromSeq  int64                      `json:"fromSeq"`
	ToSeq    int64                      `json:"toSeq"`
	Breaking bool                       `json:"breaking"`
	Changes  []ContractVersionDiffEntry `json:"changes"`
}
```

### Notes (version history)

- The criterion is exactly `UpsertContract`'s "skip write when content is unchanged" (the same `contractSemanticallyEqual`): re-registering identical content produces no history.
- Per-function retention cap `FunctionContractVersionRetention` (currently 50); the oldest entries are evicted beyond it.
- History write failures degrade to a warning and do not block registration (derived audit data; the table's existence is guaranteed by goose 0029 + startup `MinimumRequiredVersion=29`).
- `seq` has no unique index (concurrent registrations across instances may tie; reads sort stably by `seq DESC, id DESC`).

### 25. "Get all function version floors"

1. route definition

- Url: /api/v1/functions/version-floors
- Method: GET
- Response: all configured floors under the current game/env (functions without one are absent from the response)

```go
type versionFloorsListResponse struct {
	Floors map[string]string `json:"floors"` // functionId -> minVersion
}
```

### 26. "Batch set/clear function version floors"

1. route definition

- Url: /api/v1/functions/version-floor/batch
- Method: POST
- Permission: `functions:manage` (same as the single-function PUT/DELETE)
- Request:

```go
type versionFloorBatchRequest struct {
	FunctionIds []string `json:"functionIds" binding:"required"` // empty after trim + dedup returns 400
	MinVersion  string   `json:"minVersion"`                     // empty string = batch clear (equivalent to per-function DELETE)
}
```

- Response: partial-success semantics (per-function Set/Delete loop, each idempotent and retryable, no transaction)

```go
type versionFloorBatchResponse struct {
	Updated    int      `json:"updated"`
	Failed     []string `json:"failed"`
	MinVersion string   `json:"minVersion,omitempty"` // echoes the value set this time
}
```

### 27. "Get the function version history index"

1. route definition

- Url: /api/v1/functions/version-history
- Method: GET
- Response: every contract version each function has historically appeared under, for the current game/env (feeds the version-floor dropdown, OPEN-ISSUES #26; functions with no history are absent from the response)

```go
type functionVersionIndexResponse struct {
	Items []struct {
		FunctionID string   `json:"functionId"` // lexicographic order
		Versions   []string `json:"versions"`   // semver descending numerically, unparseable versions last
	} `json:"items"`
}
```

The data source is a distinct aggregation over `function_contract_versions` (30s TTL in-memory cache, no write invalidation — optional data with a bounded staleness window, following the resource-category aggregation precedent).

### Notes (version floors)

- A function-level version floor is "the minimum function version configurable per function in the UI" (compared against the function descriptor's own version, unrelated to the SDK version); the decision semantics are in `docs/architecture/data-flow.md` § "Function-level minimum version (per-function UI configuration)".
- Single-function endpoints: `GET/PUT/DELETE /api/v1/functions/:id/version-floor` (PUT body `{minVersion}`, writes need `functions:manage`; DELETE physically removes the row).
- For a non-empty batch minVersion, the server first validates the whole batch as parseable (`sdkversion.Parseable`); on invalid input it returns 400 with zero writes.
- Version floors persist in the game database's `function_version_floors` table (numbered migration 0030); the in-memory registry (no DB) degrades to a process-local map.

## Function Policy API (not wired)

The function policy (`GET/PUT/DELETE /api/v1/functions/:function_id/policy`) and system policy (`/api/v1/policies/*`) endpoints are currently **not registered in the effective routes** (`internal/handler/routes.go`; the parallel registration file mentioned by earlier docs was removed when routes were consolidated) and are not served. Policy behavior is carried by the function contract's risk/approval fields and the execution chain's governance; this section's historical documentation has been deleted and will be restored once the endpoints are wired.

## Supplementary Endpoints

The following registered endpoints are part of the function domain's canonical set:

```http
GET /api/v1/functions/{id}/analytics   # function invocation analytics
GET /api/v1/functions/instances        # all function instances
GET /api/v1/functions/warnings         # registration warning list
GET /api/v1/functions/version-floors   # all function version floors
GET /api/v1/functions/version-history  # function version history index (floor dropdown options, #26)
GET /api/v1/functions/:id/openapi      # function OpenAPI spec (public)
POST /api/v1/functions/_openapi-batch  # batch-fetch OpenAPI specs (public)
GET /api/v1/openapi/spec               # global OpenAPI document (public)
```
