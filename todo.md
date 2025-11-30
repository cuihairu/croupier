# TODO

## Service Model Coverage

The following API modules expose entities without corresponding GORM structs + DAO helpers under `services/server/internal/model`. For each item, add `<entity>.go` + `<entity>_model.go` (or equivalent) and register it in `model.AutoMigrate`.

- [x] `alert.api`: implement Alert & Silence models for alert management/muting resources.
- [x] `analytics_behavior.api`: add BehaviorEvent + FeatureAdoption structures to persist behavior analytics data.
- [x] `analytics_payments.api`: add ProductTrend + PaymentTransaction models for payment analytics.
- [x] `analytics_retention.api`: add RetentionCohort model for cohort retention tracking.
- [x] `backup.api`: add Backup model/DAO for snapshot records.
- [x] `faq.api`: add FAQ + FAQCategory models supporting knowledge base CRUD.
- [x] `feedback.api`: add Feedback model for user feedback flows.
- [x] `function.api`: add Function, FunctionDescriptor, Descriptor, FunctionInstance, FunctionPermission, PendingFunction models.
- [x] `node.api`: add Node + NodeCommand models for node inventory/execution history.
- [x] `profile.api`: add ProfilePermission + ProfileGame models (view tables for admin profile data).
- [x] `rate_limit.api`: add RateLimit model for throttling configurations.
- [x] `support.api`: add SupportTicket, SupportFAQ, SupportFeedback models.
- [x] `ticket.api`: add Ticket + Comment models for ticketing workflows.

Keep model/DAO files in `services/server/internal/model`, mirroring existing `admin.go` + `admin_model.go` pattern.

## SDK Registration Pipeline

- [x] Go SDK: replace mock `grpcManager` flow with real `LocalControlService` client + heartbeat, start the local `FunctionService` gRPC server before registration, and dispatch RPCs to registered handlers.
- [x] Agent local store: persist full `LocalFunctionDescriptor` data (id/version/service metadata) and ensure `UpstreamClient` forwards them to the server `ControlService`.
- [x] Server invoke path: implement `FunctionInvoke/StartJob/StreamJob` logic in `services/server/internal/logic/function/*` to select an agent from the registry and proxy gRPC calls via `FunctionService`.
- [x] Java SDK: mirror the Go SDK changes (real register/heartbeat + local function gRPC server) so Java services can participate in the same pipeline.
- [x] C++ SDK: remove the mock-only behavior guarded by `CROUPIER_SDK_ENABLE_GRPC` and provide a default build that uses generated protobuf stubs to register/heartbeat/start the function server.
- [x] Node.js/Python SDKs: replace placeholder clients with implementations that can register functions, expose a local gRPC server, and call the agent just like the Go/Java/C++ SDKs.
- [x] Agent gRPC daemon: add a binary (e.g. `cmd/agentd`) that boots `internal/app/agent.App`, listens on gRPC, and bridges LocalControl + FunctionService.

## SDK Capabilities & Docs

The following gaps remain before SDKs can be considered production-ready.

- [x] **Provider metadata upload**: every SDK must call `ControlService.RegisterCapabilities` with `ProviderMeta` + gzipped manifest (see `docs/control-capabilities.md`). Currently no SDK dials the control plane directly after registering with the agent.
  - [x] Go SDK: add manifest builder + ControlService client (re-use PBs under `pkg/pb/croupier/control/v1`) and expose config for the server endpoint.
  - [x] Java SDK: same as Go — manifest generation + ControlService gRPC stub invocation.
  - [x] C++ SDK: extend `GrpcClientManager` to dial the server ControlService and send manifests.
  - [x] Node.js SDK: add manifest serialization (JSON Schema from `FunctionDescriptor.input_schema/output_schema`) and gRPC client for RegisterCapabilities.
  - [x] Python SDK: same as Node.js; gzip manifest and upload provider meta.
- [x] **Documentation/examples parity**:
  - [x] Node.js SDK lacks a README that explains configuration + provides a runnable example (only `examples/main.ts` exists).
  - [x] Python SDK README still describes an async placeholder client and file upload features that are not implemented; align docs + provide an example matching the new `CroupierClient`.


## Analytics Pipeline

- [x] Analytics worker: clear `touchedDays` / `revAgg` entries after ClickHouse flush to avoid duplicate inserts and unbounded memory (internal/analytics/worker/worker.go).
- [x] Analytics worker: switch ClickHouse inserts in `internal/analytics/worker/worker.go` to reuse prepared batches and send chunks instead of `PrepareBatch` per event.
- [x] Analytics worker: support full ClickHouse DSN parsing (database, TLS, auth) so `CLICKHOUSE_DSN` is honored end-to-end.
- [x] Analytics worker: persist nonce/offset checkpoints so Redis consumer groups can resume without reprocessing entire streams.
- [x] Analytics ingest client: document the HMAC signing contract and ship helper code for web/SDKs to generate headers.
- [x] Analytics ingest dedupe: persist `(timestamp,nonce)` pairs (e.g. Redis SETEX) inside `services/ingest/cmd/root.go` to block replayed requests instead of trusting headers only.
- [x] Analytics ingest schema validation: enforce required fields per event/payment (`game_id`, `env`, `ts`, etc.) before publishing into MQ.
- [x] Analytics ingest routing: support per-game shared secrets so multi-tenant games can upload independently.

## Edge Process

- [x] Edge gRPC `FunctionService`/`JobService`/`TunnelService` under `internal/app/edge` are stubs; implement the real proxying logic to upstream agents/server.
- [x] Edge service (`services/edge`) should wire into `internal/app/edge` so HTTP/REST calls trigger the gRPC control-plane operations.

## Logic Implementation Coverage

- [x] 0 logic handlers remain as goctl scaffolds (`rg -l 'todo: add your logic here'` returns empty).
  - Server: fully implemented.
  - Agent: fully implemented.
  - Edge: fully implemented.

## Logic TODO Checklist
The following lists every logic handler that still contains the goctl placeholder. Update this checklist whenever implementations land.

### Service: Agent
#### Module: agent
- [x] services/agent/internal/logic/agent/agent_health_logic.go
- [x] services/agent/internal/logic/agent/agent_heartbeat_logic.go
- [x] services/agent/internal/logic/agent/agent_metrics_logic.go
- [x] services/agent/internal/logic/agent/agent_register_logic.go
- [x] services/agent/internal/logic/agent/function_register_logic.go
- [x] services/agent/internal/logic/agent/job_execute_logic.go
- [x] services/agent/internal/logic/agent/job_status_logic.go


### Service: Edge
#### Module: edge
- [x] services/edge/internal/logic/edge/edge_health_logic.go
- [x] services/edge/internal/logic/edge/edge_metrics_logic.go
- [x] services/edge/internal/logic/edge/proxy_logic.go
- [x] services/edge/internal/logic/edge/tunnel_close_logic.go
- [x] services/edge/internal/logic/edge/tunnel_create_logic.go
- [x] services/edge/internal/logic/edge/tunnel_list_logic.go
- [x] services/edge/internal/logic/edge/tunnel_status_logic.go


### Service: Server
#### Module: agent
- [x] services/server/internal/logic/agent/agent_analytics_filters_logic.go
- [x] services/server/internal/logic/agent/agent_meta_logic.go

#### Module: alert
- [x] services/server/internal/logic/alert/alert_silence_logic.go
- [x] services/server/internal/logic/alert/alerts_list_logic.go
- [x] services/server/internal/logic/alert/silence_delete_logic.go
- [x] services/server/internal/logic/alert/silences_list_logic.go

#### Module: analytics_behavior
- [x] services/server/internal/logic/analytics_behavior/behavior_adoption_breakdown_logic.go
- [x] services/server/internal/logic/analytics_behavior/behavior_adoption_logic.go
- [x] services/server/internal/logic/analytics_behavior/behavior_events_logic.go
- [x] services/server/internal/logic/analytics_behavior/behavior_funnel_logic.go
- [x] services/server/internal/logic/analytics_behavior/behavior_logic.go
- [x] services/server/internal/logic/analytics_behavior/behavior_paths_logic.go

#### Module: analytics_overview
- [x] services/server/internal/logic/analytics_overview/filters_get_logic.go
- [x] services/server/internal/logic/analytics_overview/filters_update_logic.go
- [x] services/server/internal/logic/analytics_overview/ingest_logic.go
- [x] services/server/internal/logic/analytics_overview/overview_logic.go
- [x] services/server/internal/logic/analytics_overview/realtime_logic.go
- [x] services/server/internal/logic/analytics_overview/realtime_series_logic.go

#### Module: analytics_payments
- [x] services/server/internal/logic/analytics_payments/payments_ingest_logic.go
- [x] services/server/internal/logic/analytics_payments/payments_logic.go
- [x] services/server/internal/logic/analytics_payments/payments_product_trend_logic.go
- [x] services/server/internal/logic/analytics_payments/payments_summary_logic.go
- [x] services/server/internal/logic/analytics_payments/payments_transactions_logic.go

#### Module: analytics_retention
- [x] services/server/internal/logic/analytics_retention/levels_episodes_logic.go
- [x] services/server/internal/logic/analytics_retention/levels_logic.go
- [x] services/server/internal/logic/analytics_retention/levels_maps_logic.go
- [x] services/server/internal/logic/analytics_retention/retention_logic.go

#### Module: approval
- [x] services/server/internal/logic/approval/approval_approve_logic.go
- [x] services/server/internal/logic/approval/approval_get_logic.go
- [x] services/server/internal/logic/approval/approval_reject_logic.go
- [x] services/server/internal/logic/approval/approvals_list_logic.go

#### Module: audit
- [x] services/server/internal/logic/audit/audit_logic.go

#### Module: auth
- [x] services/server/internal/logic/auth/logout_logic.go

#### Module: component
- [x] services/server/internal/logic/component/components_delete_logic.go
- [x] services/server/internal/logic/component/components_detail_logic.go
- [x] services/server/internal/logic/component/components_disable_logic.go
- [x] services/server/internal/logic/component/components_enable_logic.go
- [x] services/server/internal/logic/component/components_install_logic.go
- [x] services/server/internal/logic/component/components_list_logic.go
- [x] services/server/internal/logic/component/components_patch_logic.go

#### Module: config
- [x] services/server/internal/logic/config/config_upsert_logic.go
- [x] services/server/internal/logic/config/config_version_detail_logic.go
- [x] services/server/internal/logic/config/config_versions_logic.go

#### Module: feedback
- [x] services/server/internal/logic/feedback/feedback_create_logic.go
- [x] services/server/internal/logic/feedback/feedback_delete_logic.go
- [x] services/server/internal/logic/feedback/feedback_list_logic.go
- [x] services/server/internal/logic/feedback/feedback_stats_logic.go
- [x] services/server/internal/logic/feedback/feedback_update_logic.go

#### Module: meta
- [x] services/server/internal/logic/meta/root_logic.go

#### Module: storage
- [x] services/server/internal/logic/storage/signed_url_logic.go

#### Module: xrender
- [x] services/server/internal/logic/xrender/x_render_components_logic.go
- [x] services/server/internal/logic/xrender/x_render_generate_schema_logic.go
- [x] services/server/internal/logic/xrender/x_render_preview_schema_logic.go
- [x] services/server/internal/logic/xrender/x_render_templates_logic.go

#### Module: xrender_schema
- [x] services/server/internal/logic/xrender_schema/ui_schema_logic.go
