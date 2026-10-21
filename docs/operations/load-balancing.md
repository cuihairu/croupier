---
title: Load Balancing
icon: deployment
order: 7
category:
  - Operations Manual
tag:
  - deployment
  - high availability
---

# Load Balancing

The Croupier HA multi-instance architecture ([Server Multi-Instance HA (this section is Chinese-only)](/zh/architecture/server-ha-multi-instance)) has two traffic entry points, each of which needs load balancing:

- **Dashboard API (L7 / HTTP)**: the Dashboard's fronting gateway splits traffic across the Server instances on `:18780`
- **Agent access (L4 / TCP)**: the agents' in-house transport long connections (`:19090`, length-prefixed framing + multiplexing) are spread across the Server instances

**Why agents must go through an L4 LB instead of connecting directly**: with direct connections, an instance failure leaves every agent owned by that instance disconnected and requiring manual intervention; spread through an LB, reconnections are automatically distributed to live instances + re-registration updates the owner (architecture document §6 failover timeline, zero manual intervention throughout).

## Deployment Mode Constraint: Active + Cold Standby (current recommendation; the dual-active prerequisites are in place, pending switchover validation)

**Background understanding**: the agent session table and the function registration registry are **in-process memory state** of a Server instance; cross-instance sync covers only some paths (`/ops/nodes`, the agent list, and `/functions/instances` aggregate via the shared ownership table — instances was completed on 2026-09-11). The invoke execution path completed ownership-table integration and forwarding on 2026-09-11, so call correctness under dual-active no longer depends on "the request happening to land on the instance holding the agent":

- **Candidate-set fallback (RemoteAgentSource)**: when local candidates are empty or failover has exhausted the local candidates, a synchronous query of the shared ownership table + the `agent_sessions` snapshot table (1s budget; on error it degrades back to the `no live agent` semantics without amplifying the failure) supplements remote candidates; the chosen remote candidate is forwarded over the mesh to the owner instance for execution
- **Three-branch forwarding**: synchronous invoke, async tasks (start_task, the same failover as invoke — an lb routing failure moves to the next candidate for retry, failed attempts mark the task_runs row failed, exhaustion maps to 503), and task cancellation (cancel_task, resolving the agent from the shared `task_runs` on a task routing miss) all go through the same frame format (distinguished by `kind`); broadcast (candidate set local ∪ remote, deduped by AgentID, local first) also uses it; the owner side delivers directly and records the audit (no policy re-query; trust boundary in `docs/architecture/server-ha-multi-instance.md` §5.3)
- `refreshRemoteSnapshots`' 30s periodic backfill is demoted from a correctness dependency to a performance optimization layer (reduces synchronous DB lookups on the hot path)
- **Registry in-memory session lifecycle aligned with the ownership table** (completed 2026-09-11, three-level cleanup): immediate cleanup on local disconnect (the notAfter check in `RemoveAgentIfStale` blocks the race with a re-registration right at disconnect), restart recovery filters snapshot rows against the ownership table's active full set, and a 30s reconciliation pass cleans up orphan copies that "have no ownership row and no heartbeat for 5min" — disconnected agents no longer linger as zombie candidates in the function view/lb/broadcast candidate sets until ExpireAt (24h). The DB `agent_sessions` snapshot rows are neither deleted nor modified and expire naturally (the ownership table is the only entry point for cross-instance queries; snapshot rows without ownership rows have no consumers); single-instance (cluster not enabled) behavior is unchanged: disconnect-immediate cleanup still applies, and restart recovery is still full-volume

Known boundaries (read before switching to dual-active):

- the mesh interconnect is plaintext intranet TCP with no peer authentication — the interconnect port must be reachable only from the cluster intranet
- hash routing (`route: hash`) gives no stable placement across instances: extend to remote candidates only when the local candidates are exhausted
- async task events do not return over the mesh: events land in the shared DB via the owner (`task_events`), and the caller's HTTP polling/SSE reads from the shared DB (existing path unchanged)

Therefore the current deployment posture is still **active + cold standby** (dual-active switchover is a separate operation; the prerequisites are complete):

| Layer                | Configuration                              | Failure semantics                                                                                                                     |
| -------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------- |
| L4 (haproxy.cfg)     | `server croupier-server2 ... check backup` | all agents connect to the primary; after the primary is removed, agent reconnections land on the backup automatically and re-register |
| L7 (nginx-main.conf) | `split_clients ... 100% croupier-server`   | all API traffic goes to the primary; on primary failure, promote it manually here to switch to server2 (change one split value)       |

**Switching to dual-active** (prerequisites complete, timing at your discretion):

1. Change the L7 split value from `100% croupier-server` to a ratio (e.g. `50%`/`50%`), `nginx -s reload`
2. L4: remove server2's `backup` flag (agent connections start spreading)
3. Observe: both instances show agent ownership in `/ops/nodes`; invoke/async/broadcast calls triggered from the dashboard land mixed on both instances and succeed (owner-side audit records appear on the peer instance); the `server2` log shows `cluster: refreshed remote agent snapshot` / owner-side audit
4. Rollback = restore `100%` + `backup` (agents naturally return to the primary on reconnection; cross-instance calls are covered by forwarding in the meantime)

## Background Concepts: L4 / L7 / VRRP

### L4 / L7 defined by protocol layering

L4/L7 refer to the protocol layers of the OSI model — **the layer a load balancer works at determines what it can see and what it can dispatch by**:

| Layer  | OSI name    | What the LB sees                                                       | Dispatch basis                                    | Typical implementations                             |
| ------ | ----------- | ---------------------------------------------------------------------- | ------------------------------------------------- | --------------------------------------------------- |
| **L4** | transport   | IP + port (TCP/UDP headers); it does **not parse** application content | 5-tuple hash, least connections                   | LVS, HAProxy (`mode tcp`), nginx stream, cloud NLB  |
| **L7** | application | Full HTTP request (method/path/Header/Cookie)                          | Path routing, Headers, session stickiness, canary | nginx http, HAProxy (`mode http`), Envoy, cloud ALB |

The trade-off between the two:

- **L4 passes bytes through**: protocol-agnostic (in-house protocols work too), naturally friendly to long connections, low overhead; but it cannot see request content, can only dispatch at "connection" granularity, and cannot do content-based routing
- **L7 understands application semantics**: it can split/rewrite/retry/canary by path/Header; the cost is that it must parse (and even terminate) the application-layer protocol

The mapping to Croupier: the Dashboard API is standard HTTP, naturally suited to **L7**; agents use an in-house binary framing protocol (length prefix + protobuf) that the LB cannot and need not understand — **L4 passthrough** suffices — and that is the basis for the layering of the two entry points in this document.

> By the way, **LVS** (Linux Virtual Server): L4 forwarding in kernel mode (IPVS), bypassing the user-space protocol stack; the performance is the highest of all options; the cost is configuration and ecosystem complexity — usually unnecessary below a thousandish nodes within one company.

### VRRP: solving "the LB itself is a single point"

**VRRP (Virtual Router Redundancy Protocol, RFC 5798)** is a **network-layer high-availability protocol**, unrelated to load balancing itself:

- multiple hosts form one virtual router sharing a **virtual IP (VIP)**; only the VIP is exposed externally, and clients always connect to the VIP
- the protocol automatically elects a **Master** (the one with the highest priority) that holds the VIP and answers traffic; the rest are **Backup** on standby
- the Master broadcasts advertisements periodically (by default once every 1s); when a Backup misses advertisements for about 3 consecutive periods it declares the Master dead and takes over the VIP by priority — **second-level failover, imperceptible to clients**
- **keepalived** is the VRRP daemon implementation on Linux, usually deployed in pairs with nginx/HAProxy: the LB process does the dispatching, and keepalived ensures that if "the machine running the LB" dies, the VIP can drift to the backup

Note that VRRP depends on Layer-2 multicast/unicast reachability and is **unavailable inside most public-cloud VPCs** — the cloud equivalent is the cloud NLB (cross-AZ + health checks + zero ops).

## Concept Clarification: keepalived Is Not a Load Balancer

The relationship among the three is often confused:

- **nginx (stream module)** and **HAProxy**: load balancers, the ones actually dispatching traffic
- **keepalived**: the VRRP daemon, responsible only for **VIP drift** (primary/backup high availability); it does no load balancing itself; usually deployed in pairs with nginx/HAProxy to solve the "the LB itself died" problem
- a third self-hosted high-availability LB shape is **keepalived + LVS** (kernel-mode forwarding, highest performance)

## Option Comparison

### L4 Capability Comparison (nginx stream vs HAProxy)

| Dimension                 | nginx stream                                                                                                                                                                           | HAProxy                                                                                                                 | Impact on Croupier                                                                                                                                                                                       |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Runtime DNS re-resolution | ❌ upstream hostnames are resolved only at startup/reload; after an instance is rebuilt with a new IP the old IP keeps being used (502/connection failures); a manual reload is needed | ✅ `resolvers` block + `resolve-prefer ipv4`; container IP changes are followed automatically                           | **The most critical difference**: after compose rebuilds the server container, nginx needs `--force-recreate dashboard` as a crutch, while HAProxy needs no action at all                                |
| Active health checks      | ❌ the open-source edition is passive (`max_fails`/`fail_timeout`; counted only on connection failure)                                                                                 | ✅ `option tcp-check` / `http-check` active probing; removes half-dead instances                                        | Agents are long-lived multiplexed connections: when the Server process is wedged but the TCP connection is not broken, passive probing cannot remove it cleanly and calls keep hitting the dead instance |
| Graceful decommission     | ❌ reload severs existing connections outright                                                                                                                                         | ✅ runtime `set server ... state drain` via the stats socket: existing connections drain, new connections stop entering | During rolling Server upgrades, HAProxy can switch with zero dropped connections                                                                                                                         |
| Observability             | mainly the access log                                                                                                                                                                  | ✅ built-in stats page: per-backend session counts/queues/error rates visible in real time                              | When debugging "agents can't connect / connect unevenly", just look at the charts                                                                                                                        |
| Performance               | epoll, tens of thousands of concurrent connections                                                                                                                                     | epoll, same order of magnitude; both below LVS (kernel mode)                                                            | For one company with multiple games (under ~1k nodes) **neither is the bottleneck**; no need to agonize                                                                                                  |
| Component cost            | ✅ reuses the dashboard image, zero additions                                                                                                                                          | ❌ new component (image/config/monitoring surface)                                                                      | nginx's core advantage for small teams                                                                                                                                                                   |
| L7 capability             | ✅ SPA static assets + API reverse proxy + SSE tuning in one                                                                                                                           | ✅ but weaker at static asset serving than nginx                                                                        | The Dashboard already needs nginx to serve static assets                                                                                                                                                 |

### Conclusion

> **For the L4 load-balancing role, HAProxy wins across the board** (runtime re-resolution + active health checks + graceful decommission + observability); nginx stream's only advantage is **reusing the existing dashboard container, zero new components**.

Recommended choice by deployment shape:

| Deployment shape                                  | Recommendation                                                                                 | Rationale                                                                                                                                   |
| ------------------------------------------------- | ---------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| Single-host compose (`docker-compose.deploy.yml`) | **HAProxy** (current state, a standalone `haproxy` service)                                    | One uniform shape: full L4 capability (re-resolution/health checks/stats); no need for two configurations across single-host and multi-host |
| Multi-host containerized production               | **HAProxy** (the agent L4 entry point)                                                         | IP changes followed automatically + active removal of wedged instances + stats for debugging; the dashboard's L7 stays with nginx           |
| The LB itself must not be a single point          | **keepalived + HAProxy** (primary/backup VIP drift); the cloud equivalent is the **cloud NLB** | VRRP needs network-layer support; inside a public cloud use the NLB for zero ops                                                            |
| Kubernetes                                        | **Service (ClusterIP)**                                                                        | kube-proxy is a natural load balancer + Endpoints readiness probes; no need to build your own LB layer                                      |

## Configuration Examples

### nginx stream (alternative: reuse the dashboard container)

```nginx
# docker/nginx-main.conf (part of the deployment artifacts)
stream {
    upstream croupier_agent_lb {
        least_conn;                          # by active session count; fits long connections
        server croupier-server:19090;        # active + cold standby (see "Deployment Mode Constraint"):
        server croupier-server2:19090 backup; # agents reconnect to the backup after a primary failure
    }
    server {
        listen 19090;
        proxy_pass croupier_agent_lb;
        proxy_timeout 1h;                    # agent heartbeat keepalive at 30s; 1h idle fallback
        proxy_connect_timeout 3s;            # fast failover
    }
}
```

Caveats:

- upstream hostnames are resolved only at startup/reload; **after a server instance is rebuilt with a new IP**: `docker compose up -d --force-recreate dashboard`
- passive probing: a wedged instance with connections still open will not be removed; rely on the agent-side heartbeat-timeout reconnect as the fallback

### HAProxy (this repository's default, docker/configs/haproxy.cfg)

```haproxy
# /etc/haproxy/haproxy.cfg (the agent L4 entry point)
resolvers docker_dns
    nameserver dns1 127.0.0.11:53    # docker's embedded DNS; use the environment's DNS for bare metal
    resolve_retries 3
    timeout resolve 1s
    hold valid 10s                   # re-resolve every 10s; follows instance rebuilds changing IPs automatically

defaults
    mode tcp
    timeout connect 3s
    timeout client 1h                # agent long-lived sessions
    timeout server 1h

listen croupier_agent_lb
    bind *:19090
    balance leastconn
    # Active health check: TCP connect probe (the server's control listener)
    option tcp-check
    default-server inter 2s fall 3 rise 2 resolvers docker_dns init-addr libc,none
    # Active + cold standby (see "Deployment Mode Constraint"): server2 takes reconnections only after the primary is removed
    server server1 croupier-server:19090 check
    server server2 croupier-server2:19090 check backup

listen stats                        # debugging the connection distribution
    bind *:8404
    stats enable
    stats uri /stats
```

The Dashboard's L7 (API reverse proxy + SPA + SSE) stays with nginx — the two layers divide the work: nginx handles "humans" (HTTP), HAProxy handles "machines" (agent TCP).

### keepalived + HAProxy (high availability for the LB itself)

```conf
# /etc/keepalived/keepalived.conf (the primary LB host; the backup has slightly lower priority, everything else identical)
vrrp_instance VI_CROUPIER {
    state MASTER
    interface eth0
    virtual_router_id 51
    priority 100
    advert_int 1
    virtual_ipaddress {
        10.0.0.100/24                # VIP: the single address agents/Dashboard connect to
    }
}
```

Each of the two hosts runs one HAProxy (identical configurations), and keepalived keeps the VIP on the primary; if the primary dies, the VIP drifts to the backup within seconds and agent disconnects reconnect to the backup automatically — **still zero changes on the agent side**.

Note: VRRP depends on network-layer multicast/unicast support and is unavailable inside most public-cloud VPCs — the cloud equivalent is the **cloud NLB** (cross-AZ + health checks + zero ops), which directly replaces the entire keepalived+HAProxy layer.

## Zero-Change Principle on the Agent Side

However the entry layer changes shape, the agent needs no changes: single-address connect + disconnect backoff reconnect + re-register (the owner is updated to the new instance accordingly). Swapping the LB only requires pointing `server.addr` in `agent.yaml` at the new entry point.

## Migration Path (nginx stream → HAProxy)

1. Start the new HAProxy (pointing at the existing croupier-server/croupier-server2:19090)
2. Canary: first point one agent's `server.addr` at HAProxy and verify that registration and calls work
3. Switch all agents; watch the session distribution in HAProxy stats
4. Retire the nginx stream block (the dashboard returns to pure L7)

The Server cluster is oblivious throughout the whole process (the membership table updates the owner automatically by connection ownership).

## LB Monitoring (Design Decision)

> Status: Accepted (decision made; landing in phases). Core principle: **the pipeline uses the open-source ecosystem, the presentation is platform-native; data sources are configurable, bound to no LB brand.**

### Design Evolution and Conclusion

The LB monitoring approach converged over three rounds of discussion:

1. ~~a server-side in-house LB stats parsing adapter (pulls the haproxy CSV)~~ — **rejected**: reinventing the wheel, equivalent to reimplementing part of the haproxy exporter; and hard-bound to haproxy — swap the LB and it is dead
2. ~~link-out/iframe embedding of Grafana~~ — **rejected as the main plan**: a fragmented experience (a second UI), permissions bypassing the platform's RBAC (Grafana would need anonymous/auth-proxy setup)
3. **Final decision**: collection and storage use the Prometheus exporter ecosystem (open source); the presentation layer is rendered platform-native (reading the Prometheus API directly + Ant Design Charts); Grafana remains only as an external-link fallback for deep analysis

### Layered Architecture

```text
HAProxy 2.4+ built-in exporter (zero new containers, one line in the cfg)
        → Prometheus (storage/history/aggregation/alerting, open-source TSDB)
        → Server read-only proxy endpoint (metric allowlist + platform RBAC auth consolidation)
        → Dashboard native chart page /ops/lb (the same pattern as dbmon: the server pulls the data, native rendering)
```

The boundary of what we build ourselves vs. what we don't:

| Layer                      | In-house or open source             | Rationale                                                                                                      |
| -------------------------- | ----------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Collection (exporter)      | open source                         | every LB has an official/community exporter; keeping up with them all is impossible and shouldn't be attempted |
| Storage/query/alert engine | open source (Prometheus)            | building your own TSDB is pure duplicated labor                                                                |
| Query proxy + charts       | **in-house (two thin layers)**      | auth consolidation + a consistent experience; the dbmon page is the same-pattern precedent                     |
| Deep-analysis dashboard    | open source (Grafana external link) | the platform only does daily reconciliation; it does not clone Grafana                                         |

### Open-Source Exporter Comparison

| LB                 | Solution                                                                                                                                       | Integration cost           |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------- |
| HAProxy            | built-in Prometheus exporter since 2.4+ (`http-request use-service prometheus-exporter`) — natively supported by this repository's haproxy 2.9 | very low (edit the cfg)    |
| nginx              | `nginx-prometheus-exporter` (official from nginxinc)                                                                                           | low (sidecar)              |
| keepalived         | `keepalived-exporter` (VRRP state/drift)                                                                                                       | low                        |
| LVS/IPVS           | `prometheus-ipvs-exporter` (needs host privileges)                                                                                             | medium                     |
| Cloud NLB          | cloud vendor exporter (AWS yace / Alibaba Cloud aliyun-exporter)                                                                               | medium (cloud credentials) |
| Universal fallback | `blackbox_exporter` (TCP probing of entry reachability/latency — works even without a management API)                                          | very low                   |

Official Grafana dashboards: HAProxy 2 (dashboard ID 12693), NGINX (12708) — import and use.

### Core Platform-Side Charts (native rendering)

| Chart                            | PromQL (off the shelf)                                                 | Reconciliation meaning                               |
| -------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------- |
| Per-backend session distribution | `haproxy_backend_current_sessions`                                     | cross-check against the ownership table's agentCount |
| Backend health state             | `haproxy_server_status`                                                | UP/DOWN/NMA visible in real time                     |
| Request rate/error rate          | `haproxy_backend_http_requests_total` / `haproxy_backend_errors_total` | traffic anomaly detection                            |
| Health check failures            | `haproxy_server_check_failures`                                        | half-dead instance detection                         |

**Reconciliation column (zombie detector)**: `/ops/cluster` shows, per instance, "LB session count vs. ownership-table agent count" — a long-term mismatch between the two (connection alive, heartbeat stopped) is the half-open-connection signal, exactly the screenful of visibility most missing when debugging HA dual instances.

### Configurable (No Addresses Hardcoded)

```yaml
# server.yaml — deployment topology facts, overridable via environment variables (the same pattern as the cluster block)
ops:
  prometheusUrl: http://prometheus:9090 # empty = the /ops/lb page is hidden (graceful degradation for single instance / no telemetry stack)
  grafanaUrl: http://grafana:3000 # optional; when set, a "deep analysis" external link appears
```

- unset → the page does not appear, zero dependencies
- swapping the LB / adding keepalived → only change the scrape job in prometheus.yml; zero platform-side changes
- configuration ownership: the support layer (the ops block), not site configuration (business operation switches) — following the signal-layering decision

### Networking Notes (Deployment)

The telemetry stack (`docker-compose.telemetry.yaml`) and the deploy stack (`docker-compose.deploy.yml`) are **two separate compose networks**. For the Server to reach Prometheus, pick one of:

1. share an external network (declare `croupier-telemetry` as external and attach it to the deploy stack)
2. point `prometheusUrl` at a host-mapped port (`http://<host>:19092`)

The HAProxy exporter endpoint only needs to be reachable within the cluster (the scrape goes over the container network); **no host port exposure is needed** — which incidentally solves the exposed stats :8404 problem (the stats page can get basic auth or be pulled fully back into the intranet; the platform proxy goes over the exporter channel).

### Relationship to the Agent-View Three-Way Reconciliation (Proceeding in Parallel)

The Prometheus ecosystem covers the **support layer** (LB management-plane data); the platform still has to build the **business data-plane truth** itself (the ecosystem does not cover it):

```text
Three-way reconciliation:
  LB view        : the agent's TCP session is on instance X (Prometheus / exporter)
  Ownership view : instance X claims this agent, heartbeat fresh (cluster_agent_owners)
  Agent view     : the agent self-reports "I'm connected to instance X" (the registration response carries instanceId, echoed by the heartbeat)
```

Any inconsistent combination of the three precisely pinpoints the failure shape (half-open connection / route drift / lost registration). The agent view is a zero-configuration universal implementation (independent of any LB type), complementary to the Prometheus pipeline in this section.

### Landing Phases

| Phase | Content                                                                                                                                              | Dependencies                |
| ----- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------- |
| P0    | enable the exporter in haproxy.cfg + add the scrape to telemetry prometheus.yml + documentation                                                      | none (pure config)          |
| P1    | `ops.prometheusUrl` configuration + server read-only proxy endpoint (metric allowlist) + `/ops/lb` native charts + the cluster reconciliation column | P0                          |
| P2    | agent-view three-way reconciliation (registration response carries instanceId + heartbeat self-reports the owner + nodes page reconciliation column) | none (independent of P0/P1) |
| P3    | Grafana external-link configuration + blackbox_exporter as standard for data-plane probing                                                           | P1                          |
