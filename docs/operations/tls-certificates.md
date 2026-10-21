---
title: TLS and Certificates
icon: lock
order: 12
category:
  - Operations Manual
tag:
  - Security
  - Configuration
---

# TLS and Certificates

The platform's three internal links each have independent TLS configuration with no cross-effects:

| Link               | Direction                    | Server-side configuration    | Agent-side configuration                                                |
| ------------------ | ---------------------------- | ---------------------------- | ----------------------------------------------------------------------- |
| control transport  | Agent → Server `:19090`      | `control.cert / key / ca`    | `outboundTLS.*` + `server.insecure: false`                              |
| dispatch callback  | Server → Agent `httpAddr`    | `agentDispatch.toAgentTLS.*` | `tls.*` (local listener)                                                |
| Local registration | Game server → Agent `:19091` | —                            | `tls.*` (whether to encrypt is decided by the game side's requirements) |

The Dashboard's HTTP `:18780` should sit behind a fronting nginx/Ingress that terminates TLS uniformly (the platform itself does not do HTTPS).

## Development Certificates

```bash
./scripts/dev-certs.sh   # Generates self-signed certificates locally (development/integration testing only)
```

## Production Recommendations

1. **Signed by an internal CA**: issue a server certificate each for the Server transport and the Agents; distribute the CA certificate to the peers' `caFile`
2. **Mutual verification**: the Server's `control.ca` verifies Agent client certificates; the Agent's `outboundTLS.caFile` verifies the Server — mTLS on both ends
3. **Certificate rotation**: hot reloading of certificate file paths does not take effect; after rotation the corresponding process must be restarted; during a rolling window it is recommended to keep the old and new CAs side by side (`caFile` pointing at a CA bundle)
4. **Never enable `insecureSkipVerify` in production** — it bypasses identity verification, it is not a fallback for failed encryption negotiation

## Server-Side Configuration Example

```yaml
control:
  addr: ":19090"
  cert: /etc/croupier/tls/server.pem
  key: /etc/croupier/tls/server-key.pem
  ca: /etc/croupier/tls/ca.pem # Verifies Agent client certificates; empty = one-way TLS

agentDispatch:
  toAgentTLS:
    enabled: true
    certFile: /etc/croupier/tls/server.pem
    keyFile: /etc/croupier/tls/server-key.pem
    caFile: /etc/croupier/tls/ca.pem
```

## Agent-Side Configuration Example

```yaml
server:
  addr: "haproxy.internal:19090"
  insecure: false
  serverName: "croupier-transport.internal" # Specify when the certificate SAN differs from the LB address

outboundTLS:
  enabled: true
  certFile: /etc/croupier/tls/agent.pem
  keyFile: /etc/croupier/tls/agent-key.pem
  caFile: /etc/croupier/tls/ca.pem

tls: # :19091 local listener (game server → Agent)
  enabled: false # Usually plaintext within the game server's subnet; enable only if compliance requires it
```

## Certificate Validity Management

- Until the platform has built-in expiry alerts, use external patrols (Prometheus blackbox / cert-exporter) to monitor certificate `notAfter`
- The "Ops Center → Certificate Monitoring" page shows the status and expiry of certificates managed by the platform (an ops feature-domain capability, see the certificates section of the [API Reference](/api/))
- High-risk operations (dual-signature approvals) go through the platform approval flow; certificate replacement is a low-risk operation and can be executed directly

## Troubleshooting

| Symptom                                              | Common cause                                                                                         |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Agent reconnects repeatedly with `connection closed` | Certificate SAN does not include the LB address → set `server.serverName` or reissue the certificate |
| `x509: certificate signed by unknown authority`      | The peer's `caFile` has not been updated to the new CA                                               |
| Handshake succeeds but job dispatch fails            | Unrelated to TLS — check `agent.httpAddr` reachability (see [Agent Configuration](./config-agent))   |
| Plaintext connection to a TLS port                   | `server.insecure` inconsistent with the port's actual form                                           |
