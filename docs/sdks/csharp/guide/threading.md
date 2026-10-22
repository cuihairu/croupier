# Main-Thread Dispatcher

The C# SDK provides a main-thread dispatcher abstraction for handling callbacks on a specific execution context.

## Recommendations

- `Enqueue()` can be called from any thread
- `ProcessQueue()` should only be called from the controlled main loop
- Do not run long blocking work inside callbacks

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
