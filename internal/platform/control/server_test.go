package control

import (
	"bytes"
	"compress/gzip"
	"context"
	"testing"

	commonv1 "github.com/cuihairu/croupier/generated/croupier/common/v1"
	"github.com/cuihairu/croupier/generated/croupier/server/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(b)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func findDetail[T any](details []interface{}) (T, bool) {
	var zero T
	for _, d := range details {
		if v, ok := d.(T); ok {
			return v, true
		}
	}
	return zero, false
}

func TestRegisterCapabilities_InvalidManifestJSON_ReturnsInvalidArgument(t *testing.T) {
	s := NewServer(nil)
	_, err := s.RegisterCapabilities(context.Background(), &serverv1.RegisterCapabilitiesRequest{
		Provider:       &serverv1.ProviderMeta{Id: "p", Version: "1"},
		ManifestJsonGz: gzipBytes(t, []byte("{")),
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())

	details := st.Details()
	_, ok = findDetail[*errdetails.BadRequest](details)
	require.True(t, ok)
	ei, ok := findDetail[*errdetails.ErrorInfo](details)
	require.True(t, ok)
	require.Equal(t, "PROVIDER_MANIFEST_INVALID_JSON", ei.GetReason())
}

func TestRegisterCapabilities_SchemaViolation_ReturnsFieldViolations(t *testing.T) {
	s := NewServer(nil)
	_, err := s.RegisterCapabilities(context.Background(), &serverv1.RegisterCapabilitiesRequest{
		Provider:       &serverv1.ProviderMeta{Id: "p", Version: "1"},
		ManifestJsonGz: gzipBytes(t, []byte(`{"provider":{"id":"p"}}`)),
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())

	br, ok := findDetail[*errdetails.BadRequest](st.Details())
	require.True(t, ok)
	require.NotEmpty(t, br.GetFieldViolations())
}

func TestRegisterCapabilities_ValidManifest_Succeeds(t *testing.T) {
	s := NewServer(nil)
	_, err := s.RegisterCapabilities(context.Background(), &serverv1.RegisterCapabilitiesRequest{
		Provider:       &serverv1.ProviderMeta{Id: "p", Version: "1"},
		ManifestJsonGz: gzipBytes(t, []byte(`{"provider":{"id":"p","version":"1"}}`)),
	})
	require.NoError(t, err)
}

// TestNewServer 测试创建服务器
func TestNewServer(t *testing.T) {
	reg := NewServer(nil)
	if reg == nil {
		t.Fatal("NewServer should return non-nil")
	}
	if reg.Store() == nil {
		t.Error("Store() should return non-nil registry")
	}
}

// TestStore 测试获取 Store
func TestStore(t *testing.T) {
	s := NewServer(nil)
	store := s.Store()
	if store == nil {
		t.Error("Store() should return non-nil registry")
	}
}

// TestSetUpstreamClient 测试设置上游客户端
func TestSetUpstreamClient(t *testing.T) {
	s := NewServer(nil)

	// 创建一个 mock 客户端（使用 nil 作为测试）
	s.SetUpstreamClient(nil)

	// 验证设置成功（不 panic 就算成功）
	_ = s
}

// TestRegister_NilRequest 测试 nil 请求
func TestRegister_NilRequest(t *testing.T) {
	s := NewServer(nil)
	resp, err := s.Register(context.Background(), nil)
	if err != nil {
		t.Fatalf("Register(nil) error = %v", err)
	}
	if resp == nil {
		t.Error("Register(nil) should return non-nil response")
	}
}

// TestRegister_ValidRequest 测试有效请求
func TestRegister_ValidRequest(t *testing.T) {
	s := NewServer(nil)
	resp, err := s.Register(context.Background(), &serverv1.RegisterRequest{
		AgentId: "agent-1",
		GameId:  "game-1",
		Env:     "prod",
		RpcAddr: "127.0.0.1:9001",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if resp == nil {
		t.Error("Register() should return non-nil response")
	}
}

// TestRegister_WithFunctions 测试带函数注册
func TestRegister_WithFunctions(t *testing.T) {
	s := NewServer(nil)
	_, err := s.Register(context.Background(), &serverv1.RegisterRequest{
		AgentId: "agent-1",
		GameId:  "game-1",
		Env:     "prod",
		RpcAddr: "127.0.0.1:9001",
		Functions: []*serverv1.FunctionDescriptor{
			{Id: "func1", Enabled: true},
			{Id: "func2", Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
}

// TestRegister_WithProcesses 测试带进程注册
func TestRegister_WithProcesses(t *testing.T) {
	s := NewServer(nil)
	_, err := s.Register(context.Background(), &serverv1.RegisterRequest{
		AgentId: "agent-1",
		GameId:  "game-1",
		Env:     "prod",
		RpcAddr: "127.0.0.1:9001",
		Processes: []*serverv1.AgentProcess{
			{
				ServiceId:    "svc-1",
				Addr:         "127.0.0.1:8080",
				Version:      "1.0",
				LastSeenUnix: 1234567890,
				FunctionIds:  []string{"func1", "func2"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
}

// TestHeartbeat_NilRequest 测试 nil 心跳请求
func TestHeartbeat_NilRequest(t *testing.T) {
	s := NewServer(nil)
	resp, err := s.Heartbeat(context.Background(), nil)
	if err != nil {
		t.Fatalf("Heartbeat(nil) error = %v", err)
	}
	if resp == nil {
		t.Error("Heartbeat(nil) should return non-nil response")
	}
}

// TestHeartbeat_EmptyAgentId 测试空 agent ID
func TestHeartbeat_EmptyAgentId(t *testing.T) {
	s := NewServer(nil)
	resp, err := s.Heartbeat(context.Background(), &serverv1.HeartbeatRequest{})
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if resp == nil {
		t.Error("Heartbeat() should return non-nil response")
	}
}

// TestHeartbeat_ValidAgent 测试有效心跳
func TestHeartbeat_ValidAgent(t *testing.T) {
	s := NewServer(nil)

	// 先注册 agent
	_, _ = s.Register(context.Background(), &serverv1.RegisterRequest{
		AgentId: "agent-1",
		GameId:  "game-1",
		Env:     "prod",
		RpcAddr: "127.0.0.1:9001",
	})

	// 发送心跳
	resp, err := s.Heartbeat(context.Background(), &serverv1.HeartbeatRequest{
		AgentId: "agent-1",
	})
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if resp == nil {
		t.Error("Heartbeat() should return non-nil response")
	}
}

// TestRegisterCapabilities_NilRequest 测试 nil 能力注册请求
func TestRegisterCapabilities_NilRequest(t *testing.T) {
	s := NewServer(nil)
	_, err := s.RegisterCapabilities(context.Background(), nil)
	if err == nil {
		t.Error("RegisterCapabilities(nil) should return error")
	}
}

// TestRegisterCapabilities_NilProvider 测试 nil provider
func TestRegisterCapabilities_NilProvider(t *testing.T) {
	s := NewServer(nil)
	_, err := s.RegisterCapabilities(context.Background(), &serverv1.RegisterCapabilitiesRequest{})
	if err == nil {
		t.Error("RegisterCapabilities() with nil provider should return error")
	}
}

// TestRegisterCapabilities_EmptyProviderId 测试空 provider ID
func TestRegisterCapabilities_EmptyProviderId(t *testing.T) {
	s := NewServer(nil)
	_, err := s.RegisterCapabilities(context.Background(), &serverv1.RegisterCapabilitiesRequest{
		Provider: &serverv1.ProviderMeta{},
	})
	if err == nil {
		t.Error("RegisterCapabilities() with empty provider ID should return error")
	}
}

// TestDecompressManifest 测试解压缩 manifest
func TestDecompressManifest(t *testing.T) {
	s := NewServer(nil)

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"有效 gzip 数据", gzipBytes(t, []byte(`{"test":"data"}`)), false},
		{"空数据", []byte{}, true},
		{"无效 gzip", []byte("not gzip"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.decompressManifest(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("decompressManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestListFunctionsSummary_EmptyRegistry 测试空注册表
func TestListFunctionsSummary_EmptyRegistry(t *testing.T) {
	s := NewServer(nil)
	resp, err := s.ListFunctionsSummary(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListFunctionsSummary() error = %v", err)
	}
	if resp == nil {
		t.Fatal("ListFunctionsSummary() should return non-nil response")
	}
	// Functions 可以为 nil 或空切片
	if resp.Functions != nil && len(resp.Functions) != 0 {
		t.Error("Functions should be empty for empty registry")
	}
}

// TestListFunctionsSummary_WithAgents 测试带 agent 的功能列表
func TestListFunctionsSummary_WithAgents(t *testing.T) {
	s := NewServer(nil)

	// 注册 agent
	_, _ = s.Register(context.Background(), &serverv1.RegisterRequest{
		AgentId: "agent-1",
		GameId:  "game-1",
		Env:     "prod",
		RpcAddr: "127.0.0.1:9001",
		Functions: []*serverv1.FunctionDescriptor{
			{Id: "func1", Enabled: true},
			{Id: "func2", Enabled: false},
		},
	})

	resp, err := s.ListFunctionsSummary(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListFunctionsSummary() error = %v", err)
	}
	if len(resp.Functions) == 0 {
		t.Error("ListFunctionsSummary() should return functions")
	}
}

// TestParseI18n 测试解析 I18N 文本
func TestParseI18n(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		wantEn string
		wantZh string
	}{
		{"map with both", map[string]interface{}{"en": "Hello", "zh": "你好"}, "Hello", "你好"},
		{"map with en only", map[string]interface{}{"en": "Hello"}, "Hello", ""},
		{"map with zh only", map[string]interface{}{"zh": "你好"}, "", "你好"},
		{"string shortcut", "你好", "", "你好"},
		{"nil", nil, "", ""},
		{"empty map", map[string]interface{}{}, "", ""},
		{"not a map", 123, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseI18n(tt.input)
			if result == nil {
				if tt.wantEn != "" || tt.wantZh != "" {
					t.Errorf("parseI18n() = nil, want en=%q zh=%q", tt.wantEn, tt.wantZh)
				}
				return
			}
			if result.En != tt.wantEn {
				t.Errorf("parseI18n() En = %q, want %q", result.En, tt.wantEn)
			}
			if result.Zh != tt.wantZh {
				t.Errorf("parseI18n() Zh = %q, want %q", result.Zh, tt.wantZh)
			}
		})
	}
}

// TestParseStringSlice 测试解析字符串切片
func TestParseStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  []string
	}{
		{"[]interface{}", []interface{}{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"[]string", []string{"x", "y"}, []string{"x", "y"}},
		{"with empty strings", []interface{}{"a", "", "c"}, []string{"a", "c"}},
		{"nil", nil, nil},
		{"empty slice", []interface{}{}, nil},
		{"not a slice", "not a slice", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseStringSlice(tt.input)
			if len(result) != len(tt.want) {
				t.Errorf("parseStringSlice() len = %d, want %d", len(result), len(tt.want))
				return
			}
			for i, v := range result {
				if v != tt.want[i] {
					t.Errorf("parseStringSlice()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

// TestParseMenu 测试解析菜单
func TestParseMenu(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  *commonv1.Menu
	}{
		{
			"full menu",
			map[string]interface{}{
				"section": "sec",
				"group":   "grp",
				"path":    "/path",
				"order":   1.0,
				"icon":    "icon",
				"badge":   "badge",
				"hidden":  true,
			},
			&commonv1.Menu{Section: "sec", Group: "grp", Path: "/path", Order: 1, Icon: "icon", Badge: "badge", Hidden: true},
		},
		{"nil", nil, nil},
		{"not a map", "string", nil},
		{"empty map", map[string]interface{}{}, &commonv1.Menu{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMenu(tt.input)
			if tt.want == nil {
				if result != nil {
					t.Errorf("parseMenu() = %v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Errorf("parseMenu() = nil, want %v", tt.want)
				return
			}
			if result.Section != tt.want.Section {
				t.Errorf("parseMenu() Section = %q, want %q", result.Section, tt.want.Section)
			}
		})
	}
}

// TestParsePerm 测试解析权限
func TestParsePerm(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		check func(*commonv1.PermissionSpec) bool
	}{
		{
			"full permission",
			map[string]interface{}{
				"verbs":  []interface{}{"get", "list"},
				"scopes": []interface{}{"games", "functions"},
				"defaults": []interface{}{
					map[string]interface{}{"role": "admin", "verbs": []interface{}{"*"}},
				},
				"i18n_zh": map[string]interface{}{"get": "获取", "list": "列表"},
			},
			func(p *commonv1.PermissionSpec) bool {
				return len(p.Verbs) == 2 && len(p.Scopes) == 2 && len(p.Defaults) == 1 && len(p.I18NZh) == 2
			},
		},
		{"nil", nil, func(p *commonv1.PermissionSpec) bool { return p == nil }},
		{"not a map", "string", func(p *commonv1.PermissionSpec) bool { return p == nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePerm(tt.input)
			if !tt.check(result) {
				t.Errorf("parsePerm() = %v, check failed", result)
			}
		})
	}
}

// TestToFloat 测试转换为 float64
func TestToFloat(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  float64
		ok    bool
	}{
		{"float64", 3.14, 3.14, true},
		{"int", 42, 42.0, true},
		{"int32", int32(10), 10.0, true},
		{"int64", int64(100), 100.0, true},
		{"string", "not a number", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toFloat(tt.input)
			if ok != tt.ok {
				t.Errorf("toFloat() ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.want {
				t.Errorf("toFloat() = %v, want %v", result, tt.want)
			}
		})
	}
}
