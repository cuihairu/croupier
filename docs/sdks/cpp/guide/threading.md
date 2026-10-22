# Threading and Concurrency

C++ SDK integrations must explicitly control the threading model and resource lifecycles.

## Recommendations

- Keep connection threads, business threads, and background task boundaries explicit
- Protect shared state with locks or message passing
- Shutdown should explicitly drain and clean up

## Inbound Concurrency and Backpressure

Inbound requests (Agent→SDK invocations etc.) are handled by a **bounded worker pool**:
the read loop only enqueues and never runs business logic (serial execution would let one slow handler stall the whole connection).

- Default workers ≈ CPU core count; pending queue capacity ≈ `workers × 4` (absorbing 4 rounds of full bursts)
- When the business queue is full, a **busy error frame returns immediately** (`inbound queue full, retry on
another instance`), and the Agent-side failover retries on another instance — SDK memory does not accumulate
- Tuning knobs live in the transport-layer config (Go: `InboundWorkers`/`InboundQLen`;
  other languages in their own config sections), with identical semantics across the six languages

**Two-lane isolation** of control messages (heartbeat/register/drain) currently exists only in Go: control requests go through
a dedicated queue that is never rejected, so the session survives business floods (`protocol.IsControlRequest()`
classification). Other languages share one lane for heartbeat and business traffic, pending migration to the Go baseline.
