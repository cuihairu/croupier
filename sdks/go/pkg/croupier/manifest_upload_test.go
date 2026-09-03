package croupier

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	agentv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/agent/v1"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// F：控制面 manifest 上传——buildManifest 构建与 best-effort 上传。
func TestTCPManager_BuildManifest(t *testing.T) {
	config := ClientConfig{
		ProviderLang: "go",
		ProviderSDK:  "croupier-go-sdk",
	}
	manager, err := NewTCPManager(config, map[string]FunctionHandler{})
	if err != nil {
		t.Fatalf("NewTCPManager: %v", err)
	}
	m := manager.(*TCPManager)
	m.serviceID = "go-1"
	m.serviceVersion = "2.0.0"
	m.functions = []*sdkv1.ProviderFunctionDescriptor{
		{
			Id:           "player.ban",
			Version:      "1.0.0",
			Resource:     "player",
			Operation:    "ban",
			Risk:         "high",
			Permission:   "player.ban.invoke",
			Description:  "ban a player",
			InputSchema:  `{"type":"object","properties":{"id":{"type":"string"}}}`,
			OutputSchema: `{"type":"object"}`,
		},
		{Id: "  "}, // 空 id 跳过
	}

	manifest := m.buildManifest("go-1", "2.0.0", m.functions)
	provider, ok := manifest["provider"].(map[string]interface{})
	if !ok {
		t.Fatalf("provider missing: %+v", manifest)
	}
	if provider["id"] != "go-1" || provider["lang"] != "go" {
		t.Fatalf("unexpected provider: %+v", provider)
	}
	functions, ok := manifest["functions"].([]map[string]interface{})
	if !ok || len(functions) != 1 {
		t.Fatalf("expected 1 function entry, got %+v", manifest["functions"])
	}
	if functions[0]["id"] != "player.ban" {
		t.Fatalf("unexpected function: %+v", functions[0])
	}
	// schema 以原生 JSON 对象进入 manifest（非字符串）
	inputSchema, ok := functions[0]["inputSchema"].(json.RawMessage)
	if !ok {
		t.Fatalf("inputSchema should be raw JSON, got %T", functions[0]["inputSchema"])
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(inputSchema, &parsed); err != nil {
		t.Fatalf("inputSchema invalid: %v", err)
	}
}

func TestTCPManager_MaybeRegisterCapabilities(t *testing.T) {
	// 模拟控制面：读一帧（4 字节长度 + 8 字节头 + body），回确认帧
	var gotCapabilities bool
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		header := make([]byte, 4)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		size := int(header[0])<<24 | int(header[1])<<16 | int(header[2])<<8 | int(header[3])
		frameBody := make([]byte, size)
		if _, err := io.ReadFull(conn, frameBody); err != nil {
			return
		}
		req := &agentv1.RegisterCapabilitiesRequest{}
		if err := proto.Unmarshal(frameBody[protocol.HeaderSize:], req); err != nil {
			return
		}
		gzReader, err := gzip.NewReader(bytes.NewReader(req.GetManifestJsonGz()))
		if err != nil {
			return
		}
		manifestRaw, err := io.ReadAll(gzReader)
		if err != nil {
			return
		}
		var manifest map[string]interface{}
		if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
			return
		}
		if _, ok := manifest["provider"]; ok {
			gotCapabilities = true
		}
		// 回确认帧：响应 msgID + 同 reqID + 空 body
		respFrame := protocol.NewMessageBody(
			protocol.GetResponseMsgID(protocol.MsgRegisterCapabilitiesReq),
			protocol.GetMsgID(frameBody[1:4]),
			nil,
		)
		out := make([]byte, 4+len(respFrame))
		binary.BigEndian.PutUint32(out[:4], uint32(len(respFrame)))
		copy(out[4:], respFrame)
		conn.Write(out)
	}()

	config := ClientConfig{
		ControlAddr: listener.Addr().String(),
	}
	manager, err := NewTCPManager(config, map[string]FunctionHandler{})
	if err != nil {
		t.Fatalf("NewTCPManager: %v", err)
	}
	m := manager.(*TCPManager)
	m.serviceID = "go-1"
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "f1"}}

	m.maybeRegisterCapabilities("go-1", "2.0.0", m.functions)
	if !gotCapabilities {
		t.Fatal("control plane did not receive a valid manifest")
	}
}

func TestTCPManager_MaybeRegisterCapabilities_NoControlAddr(t *testing.T) {
	manager, err := NewTCPManager(ClientConfig{}, map[string]FunctionHandler{})
	if err != nil {
		t.Fatalf("NewTCPManager: %v", err)
	}
	m := manager.(*TCPManager)
	// 未配置 ControlAddr：应直接返回（无连接、无 panic）
	m.maybeRegisterCapabilities("go-1", "2.0.0", m.functions)
	if strings.TrimSpace(m.config.ControlAddr) != "" {
		t.Fatal("expected empty ControlAddr")
	}
}
