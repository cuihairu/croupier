// Package agent keeps the local gateway assembly during the sidecar migration.
package agent

import (
	"log/slog"

	"github.com/cuihairu/croupier/agents/sidecar"
)

// TaskExecutor is the sidecar task execution seam.
type TaskExecutor = sidecar.TaskExecutor

// TaskEventReporter is the sidecar task lifecycle event reporter.
type TaskEventReporter = sidecar.TaskEventReporter

// TaskRunner is the sidecar in-flight task lifecycle owner.
type TaskRunner = sidecar.TaskRunner

// NewTaskRunner forwards to the role implementation.
func NewTaskRunner(executor TaskExecutor, reporter TaskEventReporter, logger *slog.Logger) *TaskRunner {
	return sidecar.NewTaskRunner(executor, reporter, logger)
}
