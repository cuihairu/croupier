package agent

// coverage_k_final_test.go 组 K 验收补充：两个行为回归用例 + 全部残留
// 分支的可达性论证（行号对照 cover 块，详见交付报告）。
//
// 死分支（当前调用契约下不可达）：
//   - app.go:472-475  ApplyPayload 失败：ExtensionRuntime.ApplyPayload 在
//     非 nil runtime、非 nil payload 下恒返回 nil error（失败仅计入
//     result.Failed 并标记 degraded）
//   - app.go:521-523  SyncExtensionProviders 失败：该函数吞掉全部 init
//     错误（log+continue），恒 return nil
//   - app.go:550-551  fn nil/空 ID 防御：discoverExtensionFunctions 的
//     pushWithResource 只 append 非 nil 且 ID 非空的描述符
//   - app.go:697-698  sanitizeNodeKey 后 provider 名为空：ParseProviderBinding
//     成功意味着 provider 是 SanitizeKey 输出（Trim "._-" 后非空、必含
//     [a-z0-9]），sanitizeNodeKey 对同类字符集恒保留
//   - extension_external_bridge.go:46-48 provider/method 为空：
//     ParseFunctionID 返回 ok 时保证两者非空，payload 覆盖仅接受非空值
//   - extension_sync_puller.go:92-94 ApplyPayload 失败：runtime 为具体类型
//     *ExtensionRuntime，同 ApplyPayload 恒 nil 论证
//   - ops_server.go:477-479/578-580 listCronJobs 错误：
//     listCronJobsPlatform 吞掉全部 IO 错误恒 return nil
//   - systemd_parse.go:223-225 Index<0：上一行 Contains("path=") 为真时
//     Index 恒 >=0，冗余防御
//   - upstream.go:362-365 去抖定时器排水分支：Go 1.23+ timer channel 为
//     同步交付（unbuffered），fire 值要么当场交付给 select 的 timer.C
//     case（随后 timer 置 nil），要么被分支体末尾的 Reset 丢弃，不存在
//     "已 fire 未交付时选中 updateCh" 的窗口，Stop 恒返回 true
//   - app.go:180-182 Serve 错误日志：正常关闭先 close(closing) 使 Serve
//     返回 nil；非 Canceled 错误仅在 Accept 系统级故障（如 EMFILE）出现
//
// 平台/环境分支（Linux 测试环境不可达）：
//   - sysinfo.go:60-62 windows service manager 分支
//   - sysinfo.go:76 return "unknown"：需 /run/systemd 不存在；绝对路径
//     探测无注入点（本机为 systemd 环境，实验验证恒走 systemd 分支）
//   - systemd_parse.go:13-15 默认 systemdRunner 闭包：Linux 上被
//     sysinfo_linux.go 的 init() 无条件替换，默认闭包对象已丢失
//
// 产品 bug 阻塞（不可测）：
//   - ops_server.go:403-408 Stop 循环体：p.mu.Lock() 后误调 p.mu.RUnlock()
//     （写锁配读锁解锁），processes 非空时 Stop 必 fatal
//     "sync: RUnlock of unlocked RWMutex"。生产路径 Start() 登记
//     AutoRestart 进程后 Close()→Stop() 将崩溃整个 agent 进程。修复
//     （RUnlock→Unlock）后该循环体即可补测。

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- app.go:467 ApplyExtensionSyncPayloadJSON 对非法 JSON 报错并记录 ---

func TestApplyExtensionSyncPayloadJSONInvalidJSON_K(t *testing.T) {
	app := &App{extensionRuntime: NewExtensionRuntime()}

	resp, err := app.ApplyExtensionSyncPayloadJSON([]byte(`{"installations":`))
	require.Error(t, err, "malformed JSON payload must be rejected")
	assert.Nil(t, resp)

	snap := app.extensionRuntime.Snapshot()
	assert.Equal(t, "error", snap.LastApplyStatus, "unmarshal failure must be recorded via RecordError")
	assert.NotEmpty(t, snap.LastError)
}

// --- app.go:684 buildExtensionProviderEntries 跳过非 external-platform 安装 ---

func TestBuildExtensionProviderEntriesSkipsNonExternalInstallations_K(t *testing.T) {
	externalBinding := RuntimeBinding{
		BindingType: "provider",
		Spec: map[string]any{
			"provider":   "alpha",
			"operations": []any{"op1"},
		},
	}
	snap := ExtensionRuntimeSnapshot{
		Installations: []RuntimeInstallation{
			{
				// 普通（非 external-platform）扩展：即使带 provider binding
				// 也必须被 continue 跳过，不得生成 provider entry。
				ExtensionID:    "croupier.regular-extension",
				ReleaseVersion: "1.0.0",
				Bindings:       []RuntimeBinding{externalBinding},
			},
			{
				ExtensionID:    "croupier.external-platform",
				ReleaseVersion: "1.0.0",
				Bindings:       []RuntimeBinding{externalBinding},
			},
		},
	}

	entries := buildExtensionProviderEntries(snap)
	assert.Len(t, entries, 1, "only the external-platform installation yields an entry")
	entry, ok := entries["alpha"]
	require.True(t, ok, "external platform provider entry keyed by provider name")
	assert.True(t, entry.Enabled)
}
