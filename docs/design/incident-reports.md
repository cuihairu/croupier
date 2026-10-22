---
title: The Incident Reporting System — Registration, Attribution, Reports, and Leader Distribution (design brief, approved to start)
---

# The Incident Reporting System (Responsible-Party Reports × Leaderboards × Trends × Statistics)

## Status

- Status: **approved to start (the 2026-10-10 design order is on file; same-day supplementary orders merged in — see the per-section notes)** — documents land first, and work starts in §10's batches upon landing, each batch pushing only when its tests are green.
- The requirement order's four items: ① responsible-party reports (weekly/monthly, attributing incidents to a person: agent/operator/change-introduced; sources = execlog audit + herald events + incident registration) ② leaderboards (week/month/quarter/year, by incident count/impact duration/fix speed) ③ incident trends (time series, categories covering bugs and ops failures: red builds/service unavailability/probe failures/deploy failures) ④ incident statistics (totals/category shares/MTTR/recurrence rate).
- Supplementary orders: functional-domain categories (six defaults + settings-configurable persisted in DB + subcategories + delete-with-merge-into-uncategorized) → §2.1/§3; a leader per category → §2.1/§6; reports aggregated and distributed per category → §6; comparison dimensions (period-over-period/monthly/year-over-year three columns + last-year dashed overlay + missing data never faked) → §5.4; external APIs (settled: REST + Webhook only; no SDK/OpenAPI client generation; OpenAPI as docs only) → §9; the outlet provider chain (the built-in site outlet always present → external providers optional with degradation, no-op by default as the first principle) and notification tiering (level/type/source/ref/scope + urgency ordering + escalation dedup) and notification visibility (audience targeting + admin full view + fallback group) → §2.5/§6, with mechanics in [the plugin mechanism](plugin-mechanism.md) §5.1; the companion server-status provider (maintenance-window gate) has its own brief.
- Data-source reconnaissance baseline: alerts (`internal/model/alert.go`, meta DB, no game-ownership columns), execution_logs (`internal/model/execution_log.go:14`, Actor/FunctionID/Status/DurationMs, per-game DB), bugs (`internal/model/bug.go:18`, with Source/Assignee/CrashFingerprint), cicd_builds (table created by migration 0038), supervisor events (in-memory ring, exported to herald via the MetricsStore hook), probe failure windows (the availability category).

## 1. Positioning and Responsibility Boundaries

| Responsibility                                                | Owner      | Notes                                                                                                                                                                      |
| ------------------------------------------------------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Incident registration and attribution (Record)                | croupier   | The incidents table is the truth store; three entries: manual registration / external API / one-click conversion from automatic sources                                    |
| Category system management (functional domains + leaders)     | croupier   | The incident_categories config table, editable in settings; API validation = legal values from the config table                                                            |
| Report aggregation (week/month/quarter/year + PoP/YoY)        | croupier   | The report generator runs server-locally; payloads stored in incident_reports                                                                                              |
| Delivery and distribution (leader splits/channels/recipients) | **herald** | Courier-style: croupier aggregates per category and hands off to the herald outlet; recipient addresses/channels live entirely on the herald side (the existing M2 outlet) |
| The bug ticket body itself                                    | croupier   | The existing bugs system is untouched (#25 ticket linkage, crash aggregation as-is); the reporting side only adds a category column and joins                              |
| Non-goals                                                     | —          | No cross-system alert convergence/noise reduction (per the alerts.md boundary); v1 does no fully automatic incident merging (see §4.3 and the boundary list)               |

## 2. Data Model

### 2.1 incident_categories (the category config table)

```go
type IncidentCategory struct {
    gorm.Model
    Name          string `gorm:"size:64;not null;uniqueIndex:uidx_incident_categories_name"` // frontend/client/art/design/qa/ops/uncategorized
    Slug          string `gorm:"size:64;not null;uniqueIndex:uidx_incident_categories_slug"` // frontend/client/art/design/qa/ops/uncategorized
    Sort          int    // display order (panel/report ordering)
    Leader        string `gorm:"size:64;index"` // the responsible leader (account or name; empty = no leader, distribution skips)
    Subcategories JSON   `gorm:"type:json"`     // subcategory whitelist (string array; empty = unrestricted)
    Audience      JSON   `gorm:"type:json"`     // visibility mapping {roles:[], users:[]}: which department roles + people can see this category's notifications/reports (editable in settings)
    Enabled       bool   // when disabled: not selectable at registration, report history kept, dynamic charts drop the line
    Builtin       bool   // the uncategorized row = true (undeletable, slug immutable); the six seeded categories are also builtin (name/leader/subcategories editable)
}
```

- **First-start seeding** (idempotent inside the migration: insert only missing rows): `uncategorized (builtin, undeletable)` + the six categories `frontend/client/art/design/qa/ops`, with initial subcategory whitelist samples: client `[crash, performance, network]`, ops `[deploy, config, probe, build]` — the seeded values are samples; everything is editable in settings afterward.
- Incidents link by **category_id** (renames never break the chain); the API-level category check = `category_id must be an Enabled row of this table` (the enum comes from the config table, never hard-coded).
- **Delete semantics (two stages)**: a delete request first returns the incident/bug counts under the category (a cascade warning); confirmed delete = within one transaction `UPDATE incidents/bugs SET category_id=uncategorized`, then physically delete the row. The recommended path is disabling via `Enabled=false` (historical reports unaffected); physical delete exists for the "created by mistake" case.

### 2.2 incidents (the incident registration table)

```go
type Incident struct {
    gorm.Model
    IncidentKey string `gorm:"size:128;uniqueIndex:uidx_incidents_key"` // idempotency key: automatic sources = derived fingerprint; external API = Idempotency-Key/eventId; manual = nullable (auto-increment PK fallback)
    Title       string `gorm:"size:255;not null"`
    CategoryID  uint   `gorm:"index"` // → incident_categories; deleting a category merges into uncategorized
    Subcategory string `gorm:"size:64"` // must ∈ the category's Subcategories whitelist (strongly validated when the category configures one)
    Severity    string `gorm:"size:16;index"` // info|warning|critical
    Status      string `gorm:"size:16;index"` // open|acknowledged|resolved
    Source      string `gorm:"size:32;index"` // manual|external|alert|cicd|probe|supervisor|bug
    // Responsibility attribution
    ResponsibleType string `gorm:"size:16;index"` // agent|operator|change|unknown
    ResponsibleID   string `gorm:"size:255;index"` // agentId / account (operator = the acting user; change = execlog.Actor, the change introducer)
    // Timeline (the duration-metric basis)
    DetectedAt time.Time  `gorm:"index"` // occurrence/discovery time (backdating allowed; defaults to creation time)
    ResolvedAt *time.Time // written when resolved; see §5.2 for the duration and MTTR standards
    // Linkage
    GameID    string `gorm:"size:64;index"` // nullable (platform-level incidents)
    Env       string `gorm:"size:64;index"`
    ExecLogIDs JSON  `gorm:"type:json"` // linked execution_logs.ID list (change-introduction chain evidence)
    RefType    string `gorm:"size:32"`  // linked source object type: alert|cicd_build|bug|probe_window|supervisor_event
    RefID      string `gorm:"size:128;index"` // linked source object PK (alert_id / build id / …)
    Details    JSONMap `gorm:"type:json"` // free-form details (affected players, root cause, retrospective links…)
    CreatedBy  string `gorm:"size:64"`  // manual = the console operator; external = the caller's token name (§9.3)
}
```

- **Stored in the meta DB** (like alerts: cross-game aggregate reports are the primary use case; game_id is a filter column only — no per-game database split).
- State machine: `open → acknowledged → resolved` (acknowledged may be skipped); resolved allows reopening (reopening clears ResolvedAt; recurrence statistics are unaffected by reopening — see §5.3 for the recurrence standard).
- **A note on kind**: bugs and incidents are two tables (bugs is the full ticket system — no duplicate rows); the report layer joins both tables into unified aggregates (§5). The incidents.Source=bug shape exists only for the edge case of "spinning an independent incident out of a bug manually"; v1 offers no such conversion.

### 2.3 bugs Alignment (add columns, do not touch the existing system)

- Migration 0040 adds to bugs: `category_id (uint, index)` + `subcategory (varchar 64)` (the `Migrator().HasColumn` + `AddColumn` pattern, per the 0015/0016/0025/0027 precedents; **whole-model AutoMigrate is forbidden**).
- The bug registration page makes category required (the same category selector as incidents); legacy bugs with category_id=0 fall into the "uncategorized" line at the report layer.
- Report standards: a bug's detected_at=created_at and resolved_at=the moment its status enters resolved/closed (following the bugs status closed set).

### 2.4 incident_reports and External Callers (0041)

```go
type IncidentReport struct {
    gorm.Model
    PeriodType  string `gorm:"size:16;uniqueIndex:uidx_incident_reports_period"` // week|month|quarter|year
    PeriodStart string `gorm:"size:16;uniqueIndex:uidx_incident_reports_period"` // 2026-W41 | 2026-10 | 2026-Q4 | 2026
    ReportKind  string `gorm:"size:16"`  // responsibility|leaderboard|trend|summary
    Payload     JSON   `gorm:"type:json"` // the full report payload (§5 metrics + comparisons + matrix)
    GeneratedAt time.Time
    PushStatus  JSON   `gorm:"type:json"` // per-category distribution results [{slug,leader,pushedAt,eventId,ok}]
}

type ExternalToken struct {
    gorm.Model
    Name       string `gorm:"size:64;not null;uniqueIndex"` // caller identity (audit/rate-limit dimension)
    TokenHash  string `gorm:"size:64;not null;uniqueIndex"` // sha256(token) hex; plaintext returned exactly once in the create response
    Scope      JSON   `gorm:"type:json"`                    // visibility {categories:[slug...]}; null = full: filters GETs + webhook events
    Enabled    bool
    LastUsedAt *time.Time
    CreatedBy  string `gorm:"size:64"`
}
```

### 2.5 Migrations and the Three-Way Sync

- **0040**: create `incident_categories` (with the audience column) + `incidents` (CreateTable after a HasTable check, the 0019 precedent; new tables carry no legacy constraint-name drift) + add two bug columns (HasColumn+AddColumn) + seed the seven category rows (idempotent: insert by slug only when missing).
- **0041**: create `incident_reports` + `external_tokens` + **add messages columns** (level/source/ref_type/ref_id/scope; type reuses the existing column; level backfills info historically — `Migrator().HasColumn`+`AddColumn`+backfill UPDATE, landing with §6's report distribution and §9's external API batch).

**In-site notification field reinforcement (same batch as 0041)**: per the plugin-mechanism §5.1 notification reinforcement order — `level` (info|warn|critical, historical backfill info) / `source` (the agent/status source) / `ref_type`+`ref_id` (linking incident/server) / `scope` (a required visibility field); urgency ordering, visual tiering, repeated-trigger escalation dedup (warn→critical repeats) and the **notification visibility rules** (targeting by incident_categories.audience + leaders + admin full view + a fallback receiving group so nothing falls through) are all in plugin-mechanism §5.1.

- Three-way sync: `internal/svc/migrations.go` (the function + `registerSvcMigrations` registration + the header version-list comment), `internal/db/migrate/migrate.go` (MinimumRequiredVersion bump), `internal/db/migrate/migrate_test.go` (the synthetic-list probe entry + version assertion). Regression cases replicate the existing shape (the bug add-column uses the hand-written CREATE TABLE legacy-constraint-name precedent `TestComponentTemplateColumnsMigration_LegacyTableShape`).

## 3. Category Management (settings-configurable, stored in DB, never hard-coded)

- **Settings page** (console "Ops center → incident categories"): name/slug/order/leader/subcategory whitelist/enable all configurable; the slug is immutable after creation (external APIs and report payloads reference it stably), while name/leader/subcategories change freely.
- **Validation rules**: at incident/bug registration and external API submission, category_id must hit an `Enabled` row; subcategory must hit the whitelist when the category configures one, and is merely ≤64 chars when no whitelist is configured.
- **Dynamic report charting**: trend lines/pie charts/leaderboard categories are read dynamically from incident_categories (Enabled rows + the uncategorized fallback line); a new category gains a new line immediately, a disabled category leaves the current chart, and historical report payloads are never rewritten.
- **The leader field**: an account or name string (v1 has no account foreign key — herald-side audience mapping routes by that string); at most one leader per category; multi-leader scenarios resolve into audience groups on the herald side (the courier-style boundary carries over).

## 4. Incident Registration and Attribution

### 4.1 Registration Entries (three entries; category required)

| Entry                       | Path                                                                        | Category                                     | Identity trail                                       |
| --------------------------- | --------------------------------------------------------------------------- | -------------------------------------------- | ---------------------------------------------------- |
| Console registration        | the `/ops/incidents` registration form                                      | Required (an Enabled-category enum selector) | CreatedBy=the JWT operator                           |
| External REST API           | `POST /api/v1/ext/incidents` (§9)                                           | Required (slug or id)                        | CreatedBy=the token name; execution trailed in audit |
| Automatic-source conversion | the "convert to incident" button on alert/build/probe/circuit-breaker pages | Required (pre-filled suggestions, editable)  | CreatedBy=the operator; Ref* auto-filled             |

### 4.2 Attribution Field Semantics (who is the responsible party)

| ResponsibleType | Semantics                          | ResponsibleID value                            | Data source                                                                          |
| --------------- | ---------------------------------- | ---------------------------------------------- | ------------------------------------------------------------------------------------ |
| `agent`         | Introduced by an agent/game server | agentId                                        | Carried from the source event at conversion (supervisor/probe events)                |
| `operator`      | Caused by an operator's execution  | Account                                        | Taken from `execution_logs.Actor` when linking execlog                               |
| `change`        | Introduced by a change             | The change operator's account (=execlog.Actor) | The ExecLogIDs link chain: registration/publish/approval execution trails            |
| `unknown`       | Unattributed                       | Empty                                          | The registration default; reports carry an "unattributed share" to prompt completion |

- Attribution can be backfilled (unknown at registration → changed to operator/change at retrospective, with ExecLogIDs attached); responsible-party reports count the attribution snapshot as of the report period's end.

### 4.3 Automatic Sources and Conversion Mapping (v1 one-click conversion, not fully automatic)

| Automatic source                                     | Trigger surface                                        | Suggested conversion values (editable)                   | Ref* linkage             |
| ---------------------------------------------------- | ------------------------------------------------------ | -------------------------------------------------------- | ------------------------ |
| Alerts (capture/dbmon/inbound)                       | the alerts list row "convert to incident"              | severity=alert level; detected_at=the alert's fired time | RefType=alert+RefID      |
| cicd build failures (red builds)                     | the build list's failed rows "convert to incident"     | category=ops/build; ResponsibleType=change (the trigger) | RefType=cicd_build       |
| Probe failure windows (probe failures)               | the probe page's failure window "convert to incident"  | category=ops/probe; ResponsibleType=agent                | RefType=probe_window     |
| Supervisor circuit breaking (service unavailability) | the supervision events/node page "convert to incident" | category=ops; ResponsibleType=agent                      | RefType=supervisor_event |

- Honest boundary: v1 does **no automatic merging** (threshold alerts are noisy; full automation would flood the incident reports with alerts); one-click conversion keeps registration cost low. Fully automatic rules (e.g. a critical probe window auto-creating an incident) are P2, hooked to the plugin-mechanism §3.3 detector rule slot.
- **Data-source current-state anchors** (reconnoitered 2026-10-10; they shape the implementation):
  - cicd `TriggeredBy` is `api|webhook`, not a person — change attribution takes the **trigger-chain actor** (REST triggers record `Actor` in the execlog invoke) or falls back to the integration's `CreatedBy`;
  - probe failure windows (`core/healthprobe` Window has StartTS/EndTS/DurationMS — perfect MTTR material) and supervisor events (a complete event closed set) are **not persisted server-side** (agent in-memory ring/local files) — conversion must snapshot the timeline into the incident row (DetectedAt/ResolvedAt/RefID); it cannot be re-queried later;
  - the `alerts` table has no game/env/agent columns (scope lives in the Details JSON) — the incidents table's GameID/Env columns fill that gap; herald outbound events do not land in the local alerts table, so supervisor/probe-source incidents have conversion as their only entry;
  - audit_records are kept forever (the only long-horizon stream) but "deploy/change" write points are incomplete (the function-registration/pack-import operator is dropped — `internal/api/extension/service.go:596` explicitly `_ = operator`) — the primary attribution anchor is execlog.Actor (never empty);
  - **execlog's default retention is 7 days** (`internal/config/config.go:736-742`) — attribution is **snapshotted** at registration/conversion (ResponsibleID persisted), with ExecLogIDs only a traceability reference; expired logs never affect report standards.

## 5. Report Standards (every metric recomputable)

### 5.1 Period Definitions

| Tier    | Period                       | Identifier | PoP object                    | YoY object                  |
| ------- | ---------------------------- | ---------- | ----------------------------- | --------------------------- |
| Week    | ISO week (Mon 00:00, 7 days) | `2026-W41` | The previous ISO week         | The same ISO week last year |
| Month   | Calendar month               | `2026-10`  | The previous calendar month   | The same month last year    |
| Quarter | Calendar quarter             | `2026-Q4`  | The previous calendar quarter | The same quarter last year  |
| Year    | Calendar year                | `2026`     | Last year                     | (no YoY column)             |

- Time zone: **the server process's local time zone** (fixed at deployment, documented with the deployment); stored timestamps are always UTC, with period boundaries computed in local time.
- Monthly comparison = the month tier's PoP (this month vs last); it is not a separate fifth tier.

### 5.2 Metric Standards

| Metric                  | Algorithm                                                                                                                                          | Notes                                                                                     |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Incident count N        | incidents with `detected_at ∈ [period start, period end)` + bug count (both tables joined)                                                         | Including unresolved; attribution/category per the current-period snapshot                |
| Impact duration D       | `Σ(resolved_at − detected_at)`; still-open at period end counts to `min(now, period end)`                                                          | The unresolved-incident trailing standard; the report payload records the truncation time |
| MTTR                    | Resolved samples' `Σ(resolved − detected) / resolved count`                                                                                        | No resolved samples → MTTR=null (shown as "no fix samples yet" — never 0)                 |
| Fix-speed leaderboard   | MTTR ascending; only responsible parties/categories with ≥1 resolved sample included                                                               | Same as above; empty samples never enter the leaderboard                                  |
| Recurrence rate         | Recurrences / resolved count. Recurrence = the same `signature` reappearing within **7 days** after resolved (fixed v1 standard, not configurable) | Signature in §5.3                                                                         |
| Responsible-party count | Grouped and counted by ResponsibleID (unknown listed separately)                                                                                   | The responsible-party report's main column                                                |

### 5.3 Recurrence Signatures

- Automatic sources: `source + RefID derivation` (alert=rule name+target; cicd=workflow+job; probe=probe target; supervisor=agent+process name).
- Manual registration: `category + subcategory` (category alone when no subcategory).
- The same signature resolving then re-firing within the 7-day window = one recurrence; consecutive occurrences count as one recurrence chain anchored at the chain's head (chain length queryable).

### 5.4 Comparison Dimensions (PoP / monthly / YoY, three columns)

- Every metric (incident count/responsible-party counts/category shares/MTTR/impact duration/recurrence rate) outputs:

```json
{
  "value": 12,
  "prev": { "value": 8, "delta": 4, "pct": 50.0 },
  "yoy": { "value": null, "delta": null, "pct": null, "missing": true }
}
```

- PoP = the previous same-tier period; YoY = the same tier last year (same ISO week number/same month/same quarter).
- **Insufficient-data rule**: a YoY period with no data (live under a year) → `missing: true`, and the frontend shows a grey "no year-ago data" placeholder; **zero-filling and fake percentages are forbidden** (same for a zero base: when the previous period is 0, pct=null with only ±delta).
- Trend charts: the current period as a solid line + a **dashed same-period-last-year overlay** (when the yoy series lacks data the whole line is skipped, with the legend noting "no year-ago data").

### 5.5 Leaderboards (four tiers × three boards × two views)

- Tiers: week/month/quarter/year (§5.1 objects).
- Boards: ① incident count (descending) ② impact duration (descending) ③ fix speed (MTTR ascending; empty samples excluded).
- Views: two tabs — by responsible party / by category (functional domain) — making each domain's incident contribution visible at a glance (supplementary order ②).
- Every row carries three columns: current value, PoP ±value and %, YoY ±value and % (the missing rule of §5.4).

### 5.6 Responsible-Party Reports (weekly/monthly)

- Structure: report header (period/generation time/three overview metrics) → category×responsible-party matrix (how many incidents per domain, each domain's leader) → responsible-party detail rows (incident counts by category/impact duration/MTTR/recurrences + PoP and YoY) → the unattributed list (prompting completion).
- Generation timing: the weekly report generates last week every Monday 09:00 (local time); the monthly report generates last month on the 1st at 09:00; the console can **recompute on demand** per period at any time (recomputation never overwrites distributed records — it produces a new payload version).

## 6. Generation → Per-Category Aggregation → the Leader Distribution Chain

```
TaskSchedule (the task_schedules table, five-field cron:
  weekly `0 9 * * 1` / monthly `0 9 1 * *`, run-log trigger slots with idempotency windows against duplicates)
  → triggers the report generator (server-local execution, never the agent dispatch chain)
  → aggregation: incidents + bugs (§5 standards) → payload (metrics + PoP/YoY + the category matrix)
  → lands in incident_reports (Payload + GeneratedAt)
  → distribution (outlet chain = the built-in site outlet always present → external provider chain optional with degradation, plugin-mechanism §5.1):
     ① In-site notifications (the built-in default outlet, always present, zero external sending):
        · Admins receive [the total aggregate (the full incident summary)] — not one copy per person
        · Each category's leader receives [that category's split]; the category's audience (department roles + people) see it by scope
        · Notifications carry the full level/type/source/ref/scope field set (§2.5)
     ② The external outlet chain (optional providers, explicitly enabled in config; on primary-outlet failure slide to the next, terminal failures only count — never blocking):
        · kind=incident-report / severity per the tiering (below)
        · target=category-leader:<slug> (herald maps it to that leader's address/channel/IM;
          category splits without a leader are not sent externally — PushStatus records skipped(no leader), still visible in the console)
        · event_id=report:<PeriodType>:<PeriodStart>:<slug> (deterministic idempotency, with outlet-side dedup as the backstop)
  → each split's distribution result is written to incident_reports.PushStatus (slug/leader/channel/eventId/ok/error)
```

- **Push tiering** (the same rules for in-site notifications and external outlets): regular reports `info`; a category's incident count doubling PoP or recurrence over threshold → `warn`; two consecutive worsening periods or unresolved criticals → `critical` — v1 thresholds are fixed (PoP×2 / recurrence>20% / two consecutive periods), not configurable.
- **Visibility**: notifications carry scope (the category context); visibility at read time resolves via incident_categories.audience (`{roles, users}`, editable in settings) — that category's audience + its leader can see it, admins see everything; **notifications without an owning category go to "ops" or the configured fallback receiving group — never lost**.
- Read-tracking boundary: in-site message reads close the loop inside the platform (messages.ReadAt/MarkRead, the existing mechanism); herald delivery status is not written back to croupier in v1 (the herald brief's §6 boundary carries over) — read status lives on the herald side.
- Manual repush: the report detail page's "repush" button resends under the same event_id (idempotency folds duplicates) without changing the payload.

## 7. Trend and Statistics Panels (the data surface)

- Trends: bucketed by day/week/month (**bucketing scans timestamps Go-side**, per the `internal/api/analytics/invocations.go:152-194` precedent — no DATE_FORMAT/strftime, safe across postgres/sqlite dialects); series = one line per category (toggleable stacked bars/lines) + a total line + the same-period-last-year dashed line.
- Stat cards: total/category shares (pie)/MTTR/recurrence rate, each with PoP ± and YoY ± (the §5.4 missing-placeholder rule).
- Period granularity: trend buckets selectable day/week/month; leaderboards and reports tiered week/month/quarter/year.

## 8. Console Page Design (static pages, the agent-category rule)

All three pages are **static pages** (modeled on `web/src/pages/Analytics/Invocations/index.tsx`'s full-page pattern; not PageSpec dynamic pages — chart types and layout need customization, and the publish-loop closure is enforced only on the dynamic-page path, see dashboard-page-model.md):

| Page                       | Path                       | Structure                                                                                                                                                                                                                              | access       |
| -------------------------- | -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| Incident registration list | `/ops/incidents`           | Stat card row + filters (category/responsible party/status/source/time RangePicker) + a ProTable detail list + registration form + detail drawer (the SupervisorDrawer precedent: list→drawer→tabs)                                    | canOpsRead   |
| Incident reports           | `/ops/incident-reports`    | Tier tabs (week/month/quarter/year) → stat card row (three-column comparison) → trend chart (Line/stacked Column, last-year dashed) → leaderboard table (responsible party/category views) → the period responsible-party report table | canOpsRead   |
| Incident category settings | `/ops/incident-categories` | Category CRUD table (name/readonly slug/order/leader/subcategory whitelist editing/enable) + the delete-cascade warning dialog                                                                                                         | canOpsManage |

- Backend routes: the scoped group + the `FlagOps` double gate (the routes.go :210-212 pattern: L2 physically unregistered + L3 softFlags.guard 403); category settings write APIs require `ops:manage`.
- Charts: `@ant-design/charts` (the only chart library, guard-enforced); stat cards use antd Statistic + PoP/YoY arrows; wide column configs split into `columns.tsx` (the Nodes/columns.tsx precedent).
- API layer: a new `web/src/services/api/incident.ts`, with scope via `getScope()` and X-Game-ID/X-Env injected automatically by the request interceptor.
- i18n: per-page modules `web/src/locales/{zh-CN,en-US}/incidentReports.ts`, aggregated via import+spread; id convention `pages.incidentReports.*`.
- Existing page integration: the alert/build/probe pages each gain a "convert to incident" button (§4.3), minimally invasive.

## 9. External APIs (settled: REST + Webhook, two surfaces)

> Settled (user, 2026-10-10): Webhook + REST API only; SDK / openapi-generator client generation **will not be done**; the OpenAPI spec remains as interface documentation only.

### 9.1 REST Contract

| Endpoint                     | Description                                                                                                                                                                                                                                                                                                  |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `POST /api/v1/ext/incidents` | External incident registration. Headers `Authorization: Bearer <token>` + `Idempotency-Key` (or body.eventId); fields aligned to the incident model: title/category (slug or id, required)/subcategory/severity/detectedAt/responsible{type,id}/execLogIds/ref/details; source marked `external:<tokenName>` |
| `GET /api/v1/ext/incidents`  | External queries (pagination + category/status/time filters; **visibility limited by token scope**: a token without scope = full read-only; with `{categories:[...]}`, both results and that token's webhook events are limited to the category set)                                                         |
| `POST /api/v1/ext/bugs`      | External bug creation: **internally reuses the existing bug Service** (`internal/api/bug`), with a standalone external route doing token auth + a field whitelist (never exposing all internal fields); after bugs gain category_id the contract aligns with the incident model                              |

- Path decision (to correct at implementation): external endpoints live under the `/api/v1/ext/*` namespace, not §9's draft-era `/api/v1/incidents` — the console JWT routes already occupy `POST/GET /api/v1/incidents`, and gin panics at startup on same-path-same-method double registration.
- The response contract follows the CLAUDE.md API response spec (success = bare payload / error = `{error,message,details}`); idempotency: replaying the same Idempotency-Key returns the first-created result (backstopped by the `incident_key` unique index).
- A working curl is one acceptance criterion (§10 batch 5).

### 9.2 Auth / Rate Limiting / Audit

- **Tokens**: the `external_tokens` table (§2.4, sha256-hashed in DB, plaintext returned exactly once, enable/disable and revoke supported); middleware validation + `LastUsedAt` updates; **scope limits visibility**: REST query results and webhook event pushes both narrow to the token's category set (the same visibility philosophy as in-site notification scope).
- **Rate limiting**: `internal/platform/ratelimit` token bucket, bucketed per token name (default 30 req/min, configurable).
- **Audit**: writes `execution_logs` (Source=`external`, Actor=`ext:<tokenName>`, FunctionID=the operation name, RequestPayload/ResponseBody carrying payloads) + the incidents.CreatedBy double trail; sensitive fields masked per the API's existing masking.

### 9.3 Outbound Webhooks (reusing the herald outlet)

- Events: `incident.created` / `incident.escalated` (severity raised or acknowledged timed out) / `incident.resolved`.
- Envelopes use the M2 event envelope (plugin-mechanism §5.2), kind=`incident-lifecycle`, event_id=`incident:<id>:<event>` deterministic idempotency; external systems' receiving addresses/channels are configured on the herald side (courier-style — croupier configures no recipients).
- Outbound failures only log + count failures (the Outlet Manager's existing semantics), never blocking the incident main flow.

### 9.4 OpenAPI Spec (documentation only)

- `docs/openapi/incidents.yaml` (OpenAPI 3.0): a machine-readable description of the REST contract above for external self-service reading; **not wired into any code-generation pipeline**.

## 10. Batch Delivery Plan (each batch: tests green → commit → push)

| Batch   | Content                                                                                                                                                                                                                                                             | Migration | Status       |
| ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- | ------------ |
| Batch 1 | Models (categories/incidents) + migration 0040 + bug columns + category CRUD API + incident registration/list/state-transition APIs + Go tests                                                                                                                      | 0040      | ✅ 2a092ba   |
| Batch 2 | The report aggregation API: summary / trend / leaderboard / responsibility-report (on-demand recomputation, with the PoP/YoY three columns and missing semantics) + Go tests                                                                                        | —         | ✅ 11be163   |
| Batch 3 | The three console pages + the "convert to incident" button integration + bilingual i18n + jest cases + tsc + guard                                                                                                                                                  | —         | ✅ see below |
| Batch 4 | Scheduled generation (task_schedules cron) + incident_reports/external_tokens/messages columns in 0041 + in-site notification distribution (admin aggregate/leader splits/scope visibility/fallback group) + the external outlet chain's leader splits + PushStatus | 0041      | ✅ 9fd4260   |
| Batch 5 | The external REST (tokens/rate limiting/execlog external trails) + lifecycle webhooks + `docs/openapi/incidents.yaml` + a curl smoke test                                                                                                                           | —         | ✅ see below |

### Batch 3 Delivery Notes (2026-10-10)

- Three pages: `/ops/incidents` (registration/filters/detail drawer/claim/resolve/reopen), `/ops/incident-reports` (four tiers + period stepping + summary/trend/leaderboards/responsible-party reports), `/ops/incident-categories` (category CRUD + builtin protection + slug validation).
- Convert-to-incident integration: the alert page's action column (refType=alert, refId=an alertname+instance composite signature, detectedAt=startsAt) + CI/CD build-failure rows (refType=cicd_build, refId=externalId, responsibleType=change). Shared component `web/src/components/ConvertToIncidentModal`.
- Static pages bypass PageSpec (§8); the publish-loop closure is not enforced; tsc 0 errors + 5 jest suites with 23 cases + guard PASSED.
- Known boundary: probe failure windows and supervisor events are not persisted server-side (agent in-memory rings/local files, §4.3 anchors), so no list page currently hosts a conversion entry — conversion entries wait for the data surface to land.

### Batch 4 Delivery Notes (2026-10-10, 4a 46c041e + 4b 7c698ab + 4c 9fd4260 + 4d 71895a0)

- Scheduled generation: `task_schedules` seeds weekly/monthly rows (`0 9 * * 1` / `0 9 1 * *`, Kind=incident_report, idempotent by name; HasTable guards are built into the model side, silently skipping on a multiGame meta DB without tables or a bare-test ctx). The scheduler routes by Kind to server-local execution (never the agent dispatch chain; `internal/api/incident` imports svc, with reverse injection via the cmd layer's `SetReportRunner` avoiding a cycle; run-logs carry no TaskRunID, failure counting proceeds, and next fires from the current cron).
- Generation→persistence: `incident_reports` upserts by (period_type, period_start); the payload carries summary + category splits + the overall level. Tiering is a fixed standard (§6, not configurable): PoP×2 or recurrence>20% → warn; unresolved critical or two consecutive worsening periods (read from two stored historical periods — either missing means no judgment) → critical; split levels only rise, and the overall = the highest split level.
- Distribution: in-site = the generator writes messages directly via MessageSink (the aggregate lands in the fallback group `group:gm-ops`, leader splits targeted at the category leader's account, scope carrying `{"categories":[slug]}`; same-key upserts fold, levels only rise); external = `DispatchExternal` synchronously drives the external chain (stop on success/slide to the next on Retryable/terminate on Permanent, skipping the in-site outlet). With zero external config the chain holds only a noop (Delivered+channel=noop; the chain terminates silently).
- Receipts: one PushStatus per split per channel — a leader-bearing split gets two (internal + the actual terminal outlet channel); a leaderless split gets one `skipped(no leader)`; zero-volume splits are not sent; unwired outlets record `skipped(outlets not wired)`.
- API and console: `GET /api/v1/incident-reports/stored` (tier filter + pagination) + `POST /api/v1/incident-reports/:id/repush` (idempotent resend under the same event_id, payload unchanged); the reports page gains a "stored reports" section (tier filter/pagination/distribution receipt summaries/repush button), bilingual i18n + 7 jest cases.
- Visibility (4d): the `messages` read side expands audiences per §6 — admins see everything; ordinary users = direct receipts (including leader splits targeted at them) ∪ fallback-group ownership-less broadcasts (scope without categories — never lost) ∪ scope notifications of visible categories (leader ∪ audience.users ∪ audience.roles∩user roles); other people's point-to-point messages stay private. Scope filtering runs Go-side (no cross-dialect pushdown precedent for the repo's JSON columns), so audience paths pull the whole window then paginate, with `total` counted post-filter; missing dependencies (no AdminModel/no DB/no login) degrade to the existing per-recipient filtering. Disabled categories' history is kept (audience scans include disabled).
- Known boundaries: the audience path's whole-window fetch caps at `visibilityFetchCap = 1000` (GM in-site volume is tiny; tail rows beyond the cap are invisible — revisit storage-side filtering when volume grows); `UnreadCount`, SSE `Stream`, `Detail`, and `Read` still validate by recipient ownership, without audience expansion; `report_kind` is fixed to `summary` for now.

### Batch 5 Delivery Notes (2026-10-10)

- REST (§9.1): `POST/GET /api/v1/ext/incidents` + `POST /api/v1/ext/bugs` (public endpoints, routed like the cicd-webhook/alert-inbound "auth inside the handler" pattern; mounted under the ops domain's L2 flag, sharing the console incident routes' fate). External registration reuses `Service.CreateIncident` (Severity defaults to info when omitted, Source=external overridable, IncidentKey=Idempotency-Key, CreatedBy=`ext:<tokenName>`); external bugs reuse `bug.Service.Create` (ctx injects username=`ext:<tokenName>`, CreatedBy the same way).
- Auth (§9.2): external_tokens stored sha256-hashed, plaintext returned exactly once in the create response; `ResolveToken` validation (missing/unknown 401, disabled 403) + `LastUsedAt` touches. Token management endpoints `POST/GET /api/v1/ext/tokens` + `PUT/DELETE /ext/tokens/:id` mount on the protected ops group + `RequireAdminRole` (roles context containing admin passes, otherwise 403).
- Rate limiting: `ratelimit.SlidingWindowLimiter` sliding window bucketed by token name, default 30 req/min (`externalAPI.rateLimitPerMinute` configurable, ≤0 falls to default); limiter internal errors degrade to allow (a rate-limit component failure never blocks the main chain).
- Audit: the three external endpoints write `execution_logs` (Source=`external`, Actor=`ext:<tokenName>`, FunctionID=incident.create/incident.list/bug.create, request/response payloads through the writer's unified masking and truncation) + the incidents.CreatedBy double trail.
- Lifecycle webhooks (§9.3): `DispatchLifecycle` hooks after `CreateIncident` succeeds (incident.created), on `TransitionIncident` landing resolved (incident.resolved), and on `UpdateIncident` raising severity (incident.escalated, with severityRank info→warning→critical); kind=incident-lifecycle, event_id=`incident:<id>:<event>` deterministic idempotency, Severity passing the row value through; asynchronous dispatch (`go func` + `DispatchExternal`'s built-in 3 retries), failures only logged, never blocking the main flow.
- OpenAPI: `docs/openapi/incidents.yaml` (OpenAPI 3.0.3, documentation only, no code-generation pipeline).
- Known boundaries: acknowledged-timeout auto-escalation (§9.3's parenthetical scenario) is not implemented in v1 — escalated fires only from manual/external severity changes; webhook narrowing by token scope depends on herald-side audience configuration (croupier only sends events, never configures recipients); `GET /ext/incidents`'s `total` is the SQL-filtered, pre-scope-row-filtering figure (with scope narrowing, items may be fewer than total); external-source bugs' `source` normalizes to internal via normalizeSource (the bug model has no external enum), with external identity landing only in CreatedBy; in test environments sqlite's `Enabled` column default:true overrides a stored false zero value (tests write back explicitly; production disable goes through the Update path, unaffected).

## Known Boundaries (an honest list)

- v1 has no fully automatic merging: all automatic sources go through one-click conversion (§4.3); fully automatic rules are P2.
- The recurrence window is fixed at 7 days, not configurable; same-signature detection is coarse for manual registrations (category+subcategory granularity).
- The leader is a string, not an account foreign key: no account-system validation; multiple leaders resolve in outlet-side audience groups.
- Attribution snapshots depend on registration-time data: executions older than execlog's 7-day retention cannot be chain-linked after the fact (§4.3 anchors).
- Notification visibility resolves audience at read time: role/mapping changes take effect immediately, but already-distributed report split content is never rewritten. The audience expansion path caps its whole-window fetch at 1000 rows (tail messages beyond it are invisible); UnreadCount/SSE Stream/Detail/Read still validate by recipient ownership only.
- Push tiering thresholds (PoP×2 / recurrence>20% / two consecutive worsening periods) are a fixed v1 standard, not configurable.
- Herald delivery status/reads are not written back (the herald brief's §6 boundary); in-platform reads close only within in-site messages; no external outlet configured = in-site only (the no-op default, zero external sending).
- YoY is permanently missing until a year of uptime: a "no year-ago data" placeholder — never a fake number.
- External API scope narrowing is by category set (§9.2); acknowledged-timeout auto-escalation and webhook recipient configuration are not on the croupier side (see the batch 5 delivery notes); OpenAPI is docs only, no SDK generation.
- Period standards depend on the server's local time zone: multi-instance cross-time-zone deployments would drift period boundaries (the current single-instance deployment model; the scheduler shares the assumption).
- Legacy bugs have no category (category_id=0): reports place them in the "uncategorized" line, with no backfill wizard.
- Probe failure windows / supervisor events are not persisted server-side (agent in-memory rings/local files): no list page can host a "convert to incident" entry, and conversion must snapshot the timeline into the incident row (§4.3); integration waits for the data surface.
- Alerts carry no row ID (the alertmanager proxy): convert-to-incident refId uses the alertname+instance composite signature, matching the alert side's dedup standard.
