# Error Handling

All function invocations should have clear error structures and logging context.

## Advice

- Distinguish business errors from system errors by error code
- Apply a unified retry policy for connection failures, timeouts, and unavailability
- Log `FunctionId`, `CallId`, duration, and error codes
