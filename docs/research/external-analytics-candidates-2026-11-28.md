# External Professional Analytics: Candidate Matrix Survey

Date: 2026-11-28. Deliverable for DATA-010 (资料调查). The professional-need
baseline is the owner-set boundary (2026-10-29,
[core-and-external](../analytics/core-and-external.md)): complex self-service
querying/drill-down, advanced funnels/paths, attribution/ROI, predictive LTV,
and professional segmentation belong to **external** professional analytics.
This page collects official, verified facts for candidate products into an
integration matrix. **No first-integration target is selected here and no
vendor integration is claimed complete** — selection, acceptance environment,
and the integration contract (DATA-011) remain owner decisions.

Fact-collection method: official product pages and GitHub repositories fetched
2026-11-28. Cells not confirmable from official pages in this pass are marked
**NOT VERIFIED** — they must be verified against the selected product before
any contract work.

## 1. Candidate matrix

| Dimension                                                 | Metabase                                                                                                                                                    | Apache Superset                                                                                                 | Grafana OSS                                                                                                                       | PostHog                                                                                                                                                                    |
| --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Editions                                                  | Open Source (self-host Docker/JAR); Starter; Pro; Enterprise; cloud or self-hosted                                                                          | Apache project, self-hosted                                                                                     | OSS (self-managed); Cloud (managed, free tier); Enterprise (self-managed)                                                         | Cloud (free tier); self-hosted repo; `posthog-foss` purged repo                                                                                                            |
| License                                                   | OSS edition free; advanced features (SAML/JWT/OIDC SSO, row/column security) gated to Pro/Enterprise — **specific OSS license text NOT VERIFIED this pass** | Apache License 2.0 (site footer)                                                                                | **AGPL-3.0-only** with Apache-2.0 exceptions (README)                                                                             | MIT expat, except `ee/` directory (separate license) (README)                                                                                                              |
| Data input                                                | 24+ listed sources incl. **ClickHouse**, PostgreSQL, MySQL, BigQuery, Snowflake…; API/CLI/MCP programmatic access                                           | Any SQL-speaking DB; listed gallery incl. **ClickHouse**, PostgreSQL, MySQL, ClickHouse-class engines; REST API | 150+ plugins; ClickHouse plugin **NOT VERIFIED this pass** (widely listed on grafana.com plugins gallery)                         | Event-based ingestion (SDKs/autocapture/manual) + warehouse + data pipelines (README); capture API **NOT VERIFIED this pass**                                              |
| Fit to needs (funnels/paths/attribution/LTV/segmentation) | SQL-first self-service; no native product-analytics funnel/attribution models                                                                               | SQL-first self-service; no native product-analytics models                                                      | Observability dashboards; not a product-analytics tool                                                                            | Product analytics native: event analytics, group analytics, session replay, feature flags/experiments; funnels/retention listed in docs (README-level: analytics + guides) |
| Auth/SSO                                                  | JWT/SAML SSO + OIDC on **Pro/Enterprise only**; OSS basic auth (official page)                                                                              | **NOT VERIFIED this pass** (docs page not fetched)                                                              | OSS: OAuth (GitHub/Google/GitLab/Generic…), LDAP, Auth Proxy, JWT proxy; **SAML/team-sync Enterprise/Cloud only** (official docs) | **NOT VERIFIED this pass**                                                                                                                                                 |
| Embedding / deep link                                     | Embed dashboards/charts in product; multi-tenant auth, row-level permissions, white-labeling; React SDK; official page confirms                             | **NOT VERIFIED this pass**                                                                                      | Dashboard share/embed **NOT VERIFIED this pass** (docs page not fetched)                                                          | Shareable dashboards **NOT VERIFIED this pass**                                                                                                                            |
| Reverse data flow (back into croupier)                    | Query/API export only; no native push-back                                                                                                                  | REST API export only                                                                                            | API export only; data pipeline **NOT VERIFIED**                                                                                   | Data pipelines/warehouse export exist (README); contract shape **NOT VERIFIED**                                                                                            |
| Cost model                                                | Tiered (Starter/Pro/Enterprise), cloud or self-host; no dollar figures on fetched page                                                                      | Free (Apache), self-host infra cost only                                                                        | OSS free; Cloud free tier (10k metrics/50GB logs/traces…); Enterprise license                                                     | Free tier on cloud; usage-based beyond (details NOT VERIFIED)                                                                                                              |
| Deployment note for croupier                              | Reads ClickHouse directly — pairs with the existing `analytics.*` warehouse                                                                                 | Reads ClickHouse directly — pairs with existing warehouse                                                       | Reads ClickHouse via plugin — observability-shaped                                                                                | Ingests events itself — **parallel ingestion path**, not a ClickHouse reader                                                                                               |

## 2. Verified-fact highlights (2026-11-28)

- **Grafana license** is AGPL-3.0-only (Apache-2.0 exceptions) — a copyleft
  consideration if any Grafana-sourced code were ever vendored (using it as an
  external service is unaffected).
- **PostHog** is MIT except `ee/`; the `posthog-foss` repository is purged of
  proprietary features — relevant if self-hosting with a clean-license
  requirement.
- **Superset** is Apache-2.0 (cleanest license of the four) and reads
  ClickHouse natively.
- **Metabase**'s SSO and row/column security are paid-tier features — free
  edition alone cannot provide the SSO scope DATA-011's contract assumes.
- **PostHog is the only candidate that ingests events itself** (SDK/autocapture);
  the other three are readers over an existing warehouse. Adopting PostHog
  means a **second event pipeline** unless croupier exports `analytics.events`
  to it — that is a DATA-011 contract question, not a survey answer.

## 3. Boundary

- No first-integration target selected; the choice needs the owner's
  cost/deployment/ops posture (self-host vs cloud, clean-license requirement,
  warehouse-reader vs parallel-ingestion trade-off).
- Every NOT VERIFIED cell must be confirmed against the selected product's
  official docs before DATA-011 contract drafting.
- No vendor integration exists or is claimed; the extension foundations
  ([core-and-external](../analytics/core-and-external.md) §4) remain the only
  croupier-side mechanism involved.
