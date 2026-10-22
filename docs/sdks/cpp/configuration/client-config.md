# Client Config

This page is the canonical entry point for C++ SDK configuration semantics. Configuration expresses only the current unified session model; it no longer binds to the legacy local-listener, callback, or any specific transport implementation.

## Recommended Configuration Model

```json
{
  "address": "127.0.0.1:19091",
  "connectTimeoutMs": 5000,
  "requestTimeoutMs": 30000,
  "heartbeat": {
    "intervalMs": 30000
  },
  "tls": {
    "enabled": false,
    "certFile": "",
    "keyFile": "",
    "caFile": "",
    "serverName": "",
    "insecureSkipVerify": false
  },
  "reconnect": {
    "enabled": true,
    "initialDelayMs": 1000,
    "maxDelayMs": 30000,
    "multiplier": 2.0,
    "jitter": 0.2,
    "steadyStateDelayMs": 30000
  }
}
```

## Field Reference

| Field                          | Description                                             |
| ------------------------------ | ------------------------------------------------------- |
| `address`                      | Address of the Agent's local gateway                    |
| `connectTimeoutMs`             | Connection timeout                                      |
| `requestTimeoutMs`             | Per-request timeout                                     |
| `heartbeat.intervalMs`         | Provider heartbeat interval                             |
| `tls.enabled`                  | Whether TLS is enabled                                  |
| `tls.certFile`                 | Client certificate                                      |
| `tls.keyFile`                  | Client private key                                      |
| `tls.caFile`                   | CA file                                                 |
| `tls.serverName`               | Certificate verification name or SNI                    |
| `tls.insecureSkipVerify`       | Skip verification, development environments only        |
| `reconnect.enabled`            | Whether automatic reconnection is enabled               |
| `reconnect.initialDelayMs`     | Initial backoff delay                                   |
| `reconnect.maxDelayMs`         | Maximum backoff delay                                   |
| `reconnect.multiplier`         | Exponential backoff multiplier                          |
| `reconnect.jitter`             | Jitter ratio                                            |
| `reconnect.steadyStateDelayMs` | Ongoing low-cost retry interval once the cap is reached |

## Configuration Principles

- Keep the field set minimal; only keep capabilities that are explicitly needed today.
- `SDK <-> Agent` connects to the Agent's local gateway by default; TLS may stay off by default.
- Enable TLS for cross-host, cross-segment, or compliance-sensitive scenarios; enable mTLS when mutual authentication is required.
- `tls` is the security configuration of the TCP session, not a new transport kind.
- Reconnection should be on by default so a brief Agent restart does not leave the business process unavailable for long.

## Prohibited Fields

The following fields belong to the legacy callback or local-listener models and must not appear in new configurations:

- `rpc_addr`
- `local_listen`
- `grpc_target`
- `grpc_addr`
- `nng_server`
- `callback_addr`
