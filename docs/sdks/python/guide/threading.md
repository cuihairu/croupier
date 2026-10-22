# Main-Thread Dispatcher

The main-thread dispatcher ensures callbacks run on a controlled thread, reducing concurrency-related state issues.

## Basic Usage

```python
from croupier.dispatcher import MainThreadDispatcher

dispatcher = MainThreadDispatcher.get_instance()
dispatcher.initialize()

def on_response(data):
    dispatcher.enqueue(lambda: process_response(data))

while running:
    dispatcher.process_queue()
```

## Common Interface

- `initialize()`
- `enqueue(callback)`
- `enqueue_with_data(callback, data)`
- `process_queue(max_count=None)`
- `get_pending_count()`
- `is_main_thread()`
- `set_max_process_per_frame(max_count)`
- `clear()`

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
