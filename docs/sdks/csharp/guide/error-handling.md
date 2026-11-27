# Error Handling

All function invocations should have clear error structures and logging context.

## Advice

- Distinguish business errors from system errors by error code
- Apply a unified retry policy for connection failures, timeouts, and unavailability —
  but do not retry writes blindly: `RetryConfig.RetryWrites` defaults to `false`, and an
  exhausted write surfaces `InvokeResult.ErrorCode == "RESULT_UNKNOWN"`
  (or `ResultUnknownException` from `StartTaskAsync`) because the request may have
  executed before the failure. Reconcile through your own idempotency key before replaying
- Log `FunctionId`, `CallId`, duration, and error codes
