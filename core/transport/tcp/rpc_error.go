package tcp

// RPC 错误响应编码（AUD-002）。
//
// 错误响应此前在两处落点（internal/transport/tcp/server.go 的
// executionWorker 与 core/transport/tcp/mux_conn.go 的 handleInboundRequest）
// 通过字符串拼接生成 `{"error":"..."}`：错误文本里的引号 / 反斜杠 / 换行会
// 破坏 JSON，客户端 json.Unmarshal 直接失败。本文件把错误文档的编码统一到
// encoding/json（转义交给标准编码器），并按请求 msgID 生成类型对应的失败
// 响应——非 Invoke RPC 过去回空 body，protobuf 空 body 会被解码成零值响应，
// 传输层错误对客户端不可见（错误只留在服务端日志）。
//
// 诚实边界：某些响应类型（ListProcesses/ListServices/ListCronJobs/
// GetSupervisorLog/GetSystemInfo/StartTask/Heartbeat/Register 等）的 wire 契约
// 没有 error/success 通道，RPCFailureBody 对它们返回 nil（空 body），传输层
// 无法表达 handler 错误，只能记日志。这些类型在 docs/review/aud-cov-002 附注
// 中如实列出。

import (
	"encoding/json"
	"strconv"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// errorPayload 是 InvokeResponse.Payload 承载的结构化错误文档。字段顺序固定
// （Error 先、SupportedVersions 后）——既有测试断言逐字节等值
// （internal/transport/tcp/server_test.go TestServer_UnsupportedVersionRejectsRequestAndKeepsSession）。
type errorPayload struct {
	Error string `json:"error"`
	// SupportedVersions 仅在协议版本拒绝时填写，其余错误省略该键。
	SupportedVersions []int `json:"supportedVersions,omitempty"`
}

// EncodeErrorPayload 用标准 JSON 编码器渲染 `{"error": errText}` 文档。编码
// 失败（理论不可达）时回落到固定安全常量，绝不返回拼接串。
func EncodeErrorPayload(errText string) []byte {
	raw, err := json.Marshal(errorPayload{Error: errText})
	if err != nil {
		return []byte(`{"error":"rpc error"}`)
	}
	return raw
}

// EncodeVersionRejectedPayload 是协议版本拒绝专用文档：除错误文本外携带本端
// 支持的版本列表，供对端给出精确范围提示（N-1 兼容）。
func EncodeVersionRejectedPayload(version int, versions []int) []byte {
	raw, err := json.Marshal(errorPayload{
		Error:             "unsupported protocol version: " + strconv.Itoa(version),
		SupportedVersions: versions,
	})
	if err != nil {
		return []byte(`{"error":"unsupported protocol version"}`)
	}
	return raw
}

// InvokeErrorBody 是 Invoke 请求的失败响应体：InvokeResponse.Payload 承载
// 标准编码的错误文档。
func InvokeErrorBody(errText string) []byte {
	return mustMarshalInvoke(EncodeErrorPayload(errText))
}

// InvokeVersionRejectedBody 是协议版本拒绝的响应体。沿用既有 InvokeResponse
// 载体（SDK 读取 payload 并报告 supportedVersions），仅把文档编码统一到
// 标准编码器。
func InvokeVersionRejectedBody(version int, versions []int) []byte {
	return mustMarshalInvoke(EncodeVersionRejectedPayload(version, versions))
}

func mustMarshalInvoke(payload []byte) []byte {
	body, err := proto.Marshal(&sdkv1.InvokeResponse{Payload: payload})
	if err != nil {
		return nil
	}
	return body
}

// RPCFailureBody 按请求 msgID 生成该 RPC 的失败响应体：客户端能靠响应类型
// 自身的失败通道识别错误，而不是拿到零值响应（Success 被解码为 true / 列表
// 被解码为空）。返回 nil 表示该 wire 类型没有失败通道（诚实边界，见文件头）。
func RPCFailureBody(reqMsgID uint32, errText string) []byte {
	var resp proto.Message
	switch reqMsgID {
	case protocol.MsgInvokeRequest:
		return InvokeErrorBody(errText)
	case protocol.MsgGetTaskResultRequest:
		resp = &sdkv1.GetTaskResultResponse{State: "failed", Error: errText}
	case protocol.MsgRestartProcessRequest:
		resp = &opsv1.RestartProcessResponse{Success: false, Message: errText}
	case protocol.MsgStopProcessRequest:
		resp = &opsv1.StopProcessResponse{Success: false, Message: errText}
	case protocol.MsgStartProcessRequest:
		resp = &opsv1.StartProcessResponse{Success: false, Message: errText}
	case protocol.MsgUpdateProcessRequest:
		resp = &opsv1.UpdateProcessResponse{Success: false, Message: errText}
	case protocol.MsgSchedulePauseRequest:
		resp = &opsv1.SchedulePauseResponse{Success: false, Message: errText}
	case protocol.MsgPushFileRequest:
		resp = &opsv1.PushFileResponse{Success: false, Message: errText}
	case protocol.MsgExecuteCommandRequest:
		resp = &opsv1.ExecuteCommandResponse{Success: false, Error: errText}
	default:
		// 无失败通道的类型（ListProcesses/ListServices/ListCronJobs/
		// GetSupervisorLog/GetSystemInfo/StartTask/Heartbeat/Register/…）：
		// wire 契约无法表达 handler 错误，只能由调用方记日志。返回 nil
		// 保持既有空 body 语义，不引入不可解析的伪造字节。
		return nil
	}
	body, err := proto.Marshal(resp)
	if err != nil {
		return nil
	}
	return body
}
