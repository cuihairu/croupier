package io.github.cuihairu.croupier.sdk.invoker;

/** L3 Invoker extension that exposes Server-persisted task state. */
public interface TaskStatusInvoker extends Invoker {
    /** Queries {@code GET /api/v1/tasks/:id}; no local task state is fabricated. */
    TaskStatusInfo getTaskStatus(String taskId) throws InvokerException;
}
