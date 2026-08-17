package io.github.cuihairu.croupier.sdk.invoker;

/** Server-persisted task state returned by {@code GET /api/v1/tasks/:id}. */
public record TaskStatusInfo(
    String taskId,
    String functionId,
    String status,
    Integer progress,
    String message,
    String result,
    String error,
    String startedAt,
    String finishedAt,
    String createdAt,
    String updatedAt
) {}
