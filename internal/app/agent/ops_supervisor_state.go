package agent

import (
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"

	processcore "github.com/cuihairu/croupier/core/process"
)

func newManagedProcess(name string, config ManagedProcessConfig) *managedProcess {
	return &managedProcess{
		name:       name,
		config:     config,
		state:      opsv1.ProcessState_PROCESS_STATE_STOPPED,
		stopCh:     make(chan struct{}),
		supervisor: newSupervisorState(config),
	}
}

type supervisorState = processcore.SupervisorState

func newSupervisorState(config ManagedProcessConfig) *processcore.SupervisorState {
	return processcore.NewSupervisorState(restartConfig(config))
}

func breakerMessage(config ManagedProcessConfig, fails int) string {
	return processcore.BreakerMessage(restartConfig(config), int32(fails))
}
