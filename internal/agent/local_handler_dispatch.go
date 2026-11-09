package agent

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/pkg/protocol"
)

// Handle implements transportcore.Handler interface
// It dispatches requests to the appropriate handler based on message type
func (h *LocalHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	return h.handleRequest(ctx, msgID, body)
}

// handleRequest dispatches request to appropriate handler
func (h *LocalHandler) handleRequest(ctx context.Context, msgID uint32, data []byte) ([]byte, error) {
	switch msgID {
	// InvokerService
	case protocol.MsgInvokeRequest:
		return h.handleInvoke(ctx, data)
	case protocol.MsgStartTaskRequest:
		return h.handleStartTask(ctx, data)
	case protocol.MsgCancelTaskRequest:
		return h.handleCancelTask(ctx, data)

	// OpsService
	case protocol.MsgGetSystemInfoRequest:
		return h.handleGetSystemInfo(ctx, data)
	case protocol.MsgListProcessesRequest:
		return h.handleListProcesses(ctx, data)
	case protocol.MsgReportMetricsRequest:
		return h.handleReportMetrics(ctx, data)
	case protocol.MsgRestartProcessRequest:
		return h.handleRestartProcess(ctx, data)
	case protocol.MsgStopProcessRequest:
		return h.handleStopProcess(ctx, data)
	case protocol.MsgStartProcessRequest:
		return h.handleStartProcess(ctx, data)
	case protocol.MsgUpdateProcessRequest:
		return h.handleUpdateProcess(ctx, data)
	case protocol.MsgSchedulePauseRequest:
		return h.handleSchedulePause(ctx, data)
	case protocol.MsgExecuteCommandRequest:
		return h.handleExecuteCommand(ctx, data)
	case protocol.MsgPushFileRequest:
		return h.handlePushFile(ctx, data)
	case protocol.MsgListServicesRequest:
		return h.handleListServices(ctx, data)
	case protocol.MsgGetServiceStatusRequest:
		return h.handleGetServiceStatus(ctx, data)
	case protocol.MsgListCronJobsRequest:
		return h.handleListCronJobs(ctx, data)
	case protocol.MsgGetSupervisorLogRequest:
		return h.handleGetSupervisorLog(ctx, data)
	case protocol.MsgRegisterCapabilitiesReq:
		return h.handleRegisterCapabilities(ctx, data)

	// Provider session messages (for SDK Provider connections)
	case protocol.MsgProviderConnectRequest:
		return h.handleProviderConnect(ctx, data)
	case protocol.MsgProviderHeartbeatRequest:
		return h.handleProviderHeartbeat(ctx, data)
	case protocol.MsgProviderDrainRequest:
		return h.handleProviderDrain(ctx, data)

	default:
		return nil, fmt.Errorf("unknown message type: 0x%06X", msgID)
	}
}
