# Async Handlers

The C# SDK prefers `async/await`-style asynchronous handlers.

## Recommendations

- Use async APIs for all external I/O
- Do not block synchronously inside handlers
- Thread timeouts and cancellation tokens through to downstream calls
