# Concurrency and Scheduling

The Go SDK does not need a single-threaded dispatcher, but concurrency boundaries must be explicit.

## Recommendations

- Use context timeouts and cancellation
- Lock shared state or use concurrency-safe structures
- Avoid uncontrolled goroutine leaks
- Handle timeouts and retries for external dependencies uniformly

## Inbound Concurrency and Backpressure

Inbound requests (Invokes from the Agent→SDK, etc.) are handled by a **bounded worker pool**:
the read loop only enqueues work and never runs business logic (serial processing would let one slow handler stall the whole connection).

- Default worker count ≈ CPU cores; pending-queue capacity ≈ `workers × 4` (absorbs about 4 rounds of full bursts)
- When the business queue fills up, a **busy error frame is returned immediately** (`inbound queue full, retry on
another instance`) and the Agent fails over to another instance — no memory accumulation in the SDK
- Tuning lives in the transport-layer configuration (Go: `InboundWorkers`/`InboundQLen`;
  other languages: see their configuration sections), with identical semantics across all six languages

**Two-lane isolation** for control messages (heartbeat/registration/drain) is currently Go-only: control requests go
through a dedicated queue that is never rejected, so the session stays alive under business traffic surges (classified via
`protocol.IsControlRequest()`). Other languages share one lane between heartbeats and business traffic, pending migration to the Go baseline.

## Two Lanes (Go Only)

Inbound dispatch in the Go SDK uses two lanes (`transport/tcp_client.go`):

- **System lane** `ctrlInbox` (capacity 64 + single worker): heartbeat/registration/Provider session control
  (including drain) identified by `IsControlRequest()` goes here, **never rejected**
- **Business lane** `inbox` (`InboundWorkers` defaults to NumCPU, `InboundQLen` defaults to
  workers×4): fail-fast busy frame when full
- Note: `InboundWorkers`/`InboundQLen` are transport-layer configuration
  (`transport.Config`); the high-level `ClientConfig` does not forward them yet — use them when building a custom transport
- Test reference: `dual_lane_test.go` (heartbeats are still processed when the business queue is full)
