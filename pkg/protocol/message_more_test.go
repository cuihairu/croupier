package protocol

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"unsafe"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestPutGetMsgIDRoundTrip(t *testing.T) {
	cases := []uint32{0x000000, 0x000001, 0x010101, 0x04010A, 0xFFFFFF}
	buf := make([]byte, 3)
	for _, id := range cases {
		PutMsgID(buf, id)
		if got := GetMsgID(buf); got != id {
			t.Fatalf("GetMsgID(PutMsgID(%#x)) = %#x", id, got)
		}
	}
}

func TestMsgIDClassification(t *testing.T) {
	if !IsRequest(MsgInvokeRequest) {
		t.Fatal("InvokeRequest should be a request")
	}
	if IsRequest(MsgInvokeResponse) {
		t.Fatal("InvokeResponse should not be a request")
	}
	if !IsResponse(MsgInvokeResponse) {
		t.Fatal("InvokeResponse should be a response")
	}
	if IsResponse(MsgInvokeRequest) {
		t.Fatal("InvokeRequest should not be a response")
	}
	// Stream events are neither request nor response regardless of parity.
	if IsRequest(MsgTaskEvent) || IsResponse(MsgTaskEvent) {
		t.Fatal("TaskEvent must not be request/response")
	}
	if IsRequest(MsgMetricEvent) || IsResponse(MsgMetricEvent) {
		t.Fatal("MetricEvent must not be request/response")
	}
	if !IsEvent(MsgTaskEvent) || !IsEvent(MsgMetricEvent) {
		t.Fatal("TaskEvent/MetricEvent should be events")
	}
	if IsEvent(MsgInvokeRequest) || IsEvent(MsgInvokeResponse) {
		t.Fatal("Invoke frames should not be events")
	}
}

func TestControlRequestMembership(t *testing.T) {
	control := []uint32{
		MsgRegisterRequest, MsgHeartbeatRequest, MsgRegisterCapabilitiesReq,
		MsgRegisterClientRequest, MsgClientHeartbeatRequest,
		MsgProviderConnectRequest, MsgProviderHeartbeatRequest, MsgProviderDrainRequest,
		MsgServerHelloRequest,
	}
	for _, id := range control {
		if !IsControlRequest(id) {
			t.Fatalf("0x%06X should be a control request", id)
		}
	}
	business := []uint32{
		MsgInvokeRequest, MsgStartTaskRequest, MsgForwardInvokeReq,
		MsgListClientsRequest, MsgGetSystemInfoRequest, MsgProviderDrainResponse,
	}
	for _, id := range business {
		if IsControlRequest(id) {
			t.Fatalf("0x%06X should not be a control request", id)
		}
	}
}

func TestGetResponseMsgID(t *testing.T) {
	if got := GetResponseMsgID(MsgInvokeRequest); got != MsgInvokeResponse {
		t.Fatalf("GetResponseMsgID(InvokeRequest) = %#x", got)
	}
	if got := GetResponseMsgID(MsgProviderDrainRequest); got != MsgProviderDrainResponse {
		t.Fatalf("GetResponseMsgID(ProviderDrainRequest) = %#x", got)
	}
}

func TestMessageBodyRoundTrip(t *testing.T) {
	payload := []byte("hello croupier")
	body := NewMessageBody(MsgInvokeRequest, 42, payload)

	version, msgID, reqID, data, err := ParseMessageFromBody(body)
	if err != nil {
		t.Fatalf("ParseMessageFromBody: %v", err)
	}
	if version != Version1 {
		t.Fatalf("version = %d, want %d", version, Version1)
	}
	if msgID != MsgInvokeRequest {
		t.Fatalf("msgID = %#x", msgID)
	}
	if reqID != 42 {
		t.Fatalf("reqID = %d", reqID)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("data = %q", data)
	}

	// Empty payload still produces a parseable header-only body.
	empty := NewMessageBody(MsgHeartbeatRequest, 1, nil)
	if len(empty) != HeaderSize {
		t.Fatalf("header-only body length = %d", len(empty))
	}
	if _, _, _, data, err := ParseMessageFromBody(empty); err != nil || len(data) != 0 {
		t.Fatalf("empty body parse: err=%v len(data)=%d", err, len(data))
	}
}

func TestParseMessageFromBodyTooShort(t *testing.T) {
	for n := 0; n < HeaderSize; n++ {
		_, _, _, _, err := ParseMessageFromBody(make([]byte, n))
		if err == nil {
			t.Fatalf("len=%d: expected error", n)
		}
		if !strings.Contains(err.Error(), "body too short") {
			t.Fatalf("len=%d: unexpected error %v", n, err)
		}
	}
}

func TestNewMessageBodyOverflowReturnsNil(t *testing.T) {
	if got := NewMessageBody(MsgInvokeRequest, 1, make([]byte, 0)); got == nil {
		t.Fatal("normal body should not be nil")
	}
	// Forge a slice header whose length overflows HeaderSize+len(data)
	// without touching (or allocating) the backing memory.
	backing := make([]byte, 1)
	forged := unsafe.Slice(unsafe.SliceData(backing), math.MaxInt-7)
	if got := NewMessageBody(MsgInvokeRequest, 1, forged); got != nil {
		t.Fatal("overflowing body should be nil")
	}
}

func TestMsgIDStringKnownTypes(t *testing.T) {
	cases := []struct {
		msgID uint32
		want  string
	}{
		{MsgRegisterRequest, "RegisterRequest"},
		{MsgRegisterResponse, "RegisterResponse"},
		{MsgHeartbeatRequest, "HeartbeatRequest"},
		{MsgHeartbeatResponse, "HeartbeatResponse"},
		{MsgRegisterCapabilitiesReq, "RegisterCapabilitiesRequest"},
		{MsgRegisterCapabilitiesResp, "RegisterCapabilitiesResponse"},
		{MsgRegisterClientRequest, "RegisterClientRequest"},
		{MsgRegisterClientResponse, "RegisterClientResponse"},
		{MsgClientHeartbeatRequest, "ClientHeartbeatRequest"},
		{MsgClientHeartbeatResponse, "ClientHeartbeatResponse"},
		{MsgListClientsRequest, "ListClientsRequest"},
		{MsgListClientsResponse, "ListClientsResponse"},
		{MsgGetTaskResultRequest, "GetTaskResultRequest"},
		{MsgGetTaskResultResponse, "GetTaskResultResponse"},
		{MsgInvokeRequest, "InvokeRequest"},
		{MsgInvokeResponse, "InvokeResponse"},
		{MsgStartTaskRequest, "StartTaskRequest"},
		{MsgStartTaskResponse, "StartTaskResponse"},
		{MsgStreamTaskRequest, "StreamTaskRequest"},
		{MsgTaskEvent, "TaskEvent"},
		{MsgCancelTaskRequest, "CancelTaskRequest"},
		{MsgCancelTaskResponse, "CancelTaskResponse"},
		{MsgGetSystemInfoRequest, "GetSystemInfoRequest"},
		{MsgGetSystemInfoResponse, "GetSystemInfoResponse"},
		{MsgListProcessesRequest, "ListProcessesRequest"},
		{MsgListProcessesResponse, "ListProcessesResponse"},
		{MsgReportMetricsRequest, "ReportMetricsRequest"},
		{MsgReportMetricsResponse, "ReportMetricsResponse"},
		{MsgStreamMetricsRequest, "StreamMetricsRequest"},
		{MsgMetricEvent, "MetricEvent"},
		{MsgRestartProcessRequest, "RestartProcessRequest"},
		{MsgRestartProcessResponse, "RestartProcessResponse"},
		{MsgStopProcessRequest, "StopProcessRequest"},
		{MsgStopProcessResponse, "StopProcessResponse"},
		{MsgStartProcessRequest, "StartProcessRequest"},
		{MsgStartProcessResponse, "StartProcessResponse"},
		{MsgExecuteCommandRequest, "ExecuteCommandRequest"},
		{MsgExecuteCommandResponse, "ExecuteCommandResponse"},
		{MsgListServicesRequest, "ListServicesRequest"},
		{MsgListServicesResponse, "ListServicesResponse"},
		{MsgGetServiceStatusRequest, "GetServiceStatusRequest"},
		{MsgGetServiceStatusResponse, "GetServiceStatusResponse"},
		{MsgProviderConnectRequest, "ProviderConnectRequest"},
		{MsgProviderConnectResponse, "ProviderConnectResponse"},
		{MsgProviderHeartbeatRequest, "ProviderHeartbeatRequest"},
		{MsgProviderHeartbeatResponse, "ProviderHeartbeatResponse"},
		{MsgProviderDrainRequest, "ProviderDrainRequest"},
		{MsgProviderDrainResponse, "ProviderDrainResponse"},
	}
	for _, tt := range cases {
		if got := MsgIDString(tt.msgID); got != tt.want {
			t.Fatalf("MsgIDString(0x%06X) = %q, want %q", tt.msgID, got, tt.want)
		}
	}
}

func TestMsgIDStringUnknown(t *testing.T) {
	if got := MsgIDString(0xDEAD00); !strings.HasPrefix(got, "Unknown(") {
		t.Fatalf("MsgIDString(unknown) = %q", got)
	}
	if got := MsgIDString(0xDEAD00); !strings.Contains(got, "0xDEAD00") {
		t.Fatalf("MsgIDString(unknown) = %q, want hex id", got)
	}
}

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	r.Register(MsgHeartbeatRequest, func() proto.Message { return wrapperspb.String("hb") })
	r.RegisterBatch(map[uint32]func() proto.Message{
		MsgInvokeRequest: func() proto.Message { return wrapperspb.Bytes(nil) },
	})
	return r
}

func TestRegistryCreateAndUnmarshal(t *testing.T) {
	r := newTestRegistry(t)

	msg, err := r.Create(MsgHeartbeatRequest)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	val, ok := msg.(*wrapperspb.StringValue)
	if !ok || val.GetValue() != "hb" {
		t.Fatalf("Create returned unexpected message %#v", msg)
	}

	if _, err := r.Create(MsgGetSystemInfoRequest); err == nil {
		t.Fatal("unknown msgID should error")
	} else if !strings.Contains(err.Error(), "unknown message type") {
		t.Fatalf("unexpected error %v", err)
	}

	payload, err := proto.Marshal(wrapperspb.String("payload"))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	decoded, err := r.Unmarshal(MsgHeartbeatRequest, payload)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := decoded.(*wrapperspb.StringValue).GetValue(); got != "payload" {
		t.Fatalf("Unmarshal value = %q", got)
	}

	if _, err := r.Unmarshal(MsgGetSystemInfoRequest, payload); err == nil {
		t.Fatal("Unmarshal unknown msgID should error")
	}

	// Corrupt payload fails proto unmarshal with message-name context.
	if _, err := r.Unmarshal(MsgHeartbeatRequest, []byte{0xFF}); err == nil {
		t.Fatal("corrupt payload should error")
	} else if !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRegistryMustRegister(t *testing.T) {
	r := newTestRegistry(t)

	r.MustRegister(MsgGetSystemInfoRequest, func() proto.Message { return wrapperspb.Bool(true) })
	if _, err := r.Create(MsgGetSystemInfoRequest); err != nil {
		t.Fatalf("Create after MustRegister: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("duplicate MustRegister should panic")
		}
	}()
	r.MustRegister(MsgHeartbeatRequest, func() proto.Message { return wrapperspb.Bool(true) })
}
