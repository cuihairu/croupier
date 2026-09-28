package ops

// 系统维护板块（OPEN-ISSUES #52）：运行版本/启动时间/在线时长快照 +
// 检查更新入口。
//
// 「自动更新」按最小可行收口：本文件只做「检查 + 版本注记」，不自动执行
// 升级（升级涉及二进制替换/重启编排，另行立项）。检查源为可选的 L3 配置
// system.updateCheckUrl：未配置时返回版本注记（checked=false）；已配置时
// 拉取 JSON 版本清单（识别 version/tagName/tag_name/latestVersion 任意一
// 键，兼容 GitHub releases/latest）与当前版本做点分数值比较。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/secguard"
	"github.com/cuihairu/croupier/internal/svc"
)

// SystemRuntimeResponse 是 GET /api/v1/ops/system/runtime 的响应。
type SystemRuntimeResponse struct {
	Version       string `json:"version"`
	GitCommit     string `json:"gitCommit"`
	BuildTime     string `json:"buildTime"`
	StartedAt     string `json:"startedAt"` // RFC3339；进程启动时间未知时为空
	UptimeSeconds int64  `json:"uptimeSeconds"`
}

// SystemCheckUpdateResponse 是 POST /api/v1/ops/system/check-update 的响应。
// Checked 仅在成功取得远端版本时为 true；HasUpdate 仅在版本可比较且远端
// 更新时为 true。Note 始终是人读注记（前端展示原样透出）。
type SystemCheckUpdateResponse struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	HasUpdate      bool   `json:"hasUpdate"`
	Checked        bool   `json:"checked"`
	CheckedAt      string `json:"checkedAt"`
	Note           string `json:"note"`
}

// 可注入缝隙（同 Service 顶部函数变量化口径）：更新源读取与远端拉取。
var (
	systemUpdateCheckURLFn = func() string {
		v, _, ok := settings.Current().GetString(context.Background(), settings.KeySystemUpdateCheckURL)
		if !ok {
			return ""
		}
		return strings.TrimSpace(v)
	}
	systemUpdateFetchFn = fetchRemoteVersion
)

func systemRuntime(svcCtx *svc.ServiceContext) *SystemRuntimeResponse {
	resp := &SystemRuntimeResponse{}
	if svcCtx != nil {
		resp.Version = svcCtx.ServerVersion
		resp.GitCommit = svcCtx.ServerGitCommit
		resp.BuildTime = svcCtx.ServerBuildTime
		if !svcCtx.StartTime.IsZero() {
			resp.StartedAt = svcCtx.StartTime.Format(time.RFC3339)
			resp.UptimeSeconds = int64(time.Since(svcCtx.StartTime).Seconds())
		}
	}
	// ldflags 兜底（测试手工构造 svcCtx 常为空；生产两处值一致）
	if resp.Version == "" {
		resp.Version = svc.ServerVersion
	}
	if resp.GitCommit == "" {
		resp.GitCommit = svc.ServerGitCommit
	}
	if resp.BuildTime == "" {
		resp.BuildTime = svc.ServerBuildTime
	}
	return resp
}

func systemCheckUpdate(ctx context.Context, svcCtx *svc.ServiceContext) *SystemCheckUpdateResponse {
	resp := &SystemCheckUpdateResponse{
		CurrentVersion: systemRuntime(svcCtx).Version,
		CheckedAt:      time.Now().Format(time.RFC3339),
	}
	resp.Note = "自动更新未实现：本入口仅做版本检查，不自动执行升级（#52 最小收口）。"
	url := systemUpdateCheckURLFn()
	if url == "" {
		resp.Note = "未配置更新检查源（settings: system.updateCheckUrl）。" + resp.Note
		return resp
	}
	remote, err := systemUpdateFetchFn(ctx, url)
	if err != nil {
		resp.Note = "更新检查失败：" + err.Error() + "。" + resp.Note
		return resp
	}
	resp.Checked = true
	resp.LatestVersion = remote
	cur, errCur := parseVersionParts(resp.CurrentVersion)
	lat, errLat := parseVersionParts(remote)
	switch {
	case errCur != nil || errLat != nil:
		resp.Note = fmt.Sprintf("版本号无法比较（当前 %s / 远端 %s）。%s", resp.CurrentVersion, remote, resp.Note)
	case compareVersions(lat, cur) > 0:
		resp.HasUpdate = true
		resp.Note = fmt.Sprintf("发现新版本 %s（当前 %s）。%s", remote, resp.CurrentVersion, resp.Note)
	default:
		resp.Note = fmt.Sprintf("当前已是最新版本（%s）。%s", resp.CurrentVersion, resp.Note)
	}
	return resp
}

// fetchRemoteVersion 拉取更新源并以字符串返回远端版本号。
// 出站安全守卫（OPEN-ISSUES #56）：sec.* 开启时拦截受限目标。
// 出站调用策略（OPEN-ISSUES #57）：net.* 超时/重试/退避接线（GET 幂等可安全重试）。
func fetchRemoteVersion(ctx context.Context, url string) (string, error) {
	guard := secguard.Resolve(settings.Current())
	if err := secguard.CheckURL(ctx, guard, url); err != nil {
		return "", err
	}
	client := secguard.HTTPClient(guard, &http.Client{Timeout: guard.TimeoutOrDefault(5 * time.Second)})
	httpResp, err := secguard.DoWithRetry(ctx, client, http.MethodGet, url, nil, nil, guard.Retries(), guard.Backoff())
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("更新源返回 %d", httpResp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	version, err := extractVersionFromManifest(body)
	if err != nil {
		return "", err
	}
	return version, nil
}

// extractVersionFromManifest 从 JSON 清单提取版本号：version/tagName/
// tag_name/latestVersion 任一非空字符串键（tag_name 兼容 GitHub
// releases/latest），非 JSON 或全部缺失时报错。
func extractVersionFromManifest(body []byte) (string, error) {
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		return "", fmt.Errorf("更新源响应不是 JSON 版本清单")
	}
	for _, key := range []string{"version", "tagName", "tag_name", "latestVersion"} {
		if v, ok := manifest[key]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s), nil
			}
		}
	}
	return "", fmt.Errorf("更新源清单缺少版本字段（version/tagName/tag_name/latestVersion）")
}

// versionParts 是点分版本号的数值段。
type versionParts []int

// parseVersionParts 解析 "v1.2.3" 形态的点分版本号；任一段非数字即报错
// （此时检查端不猜大小，诚实返回「无法比较」）。
func parseVersionParts(v string) (versionParts, error) {
	v = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(v), "v"), "V")
	if v == "" {
		return nil, fmt.Errorf("空版本号")
	}
	parts := strings.Split(v, ".")
	out := make(versionParts, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("版本段 %q 不是数字", p)
		}
		out = append(out, n)
	}
	return out, nil
}

// compareVersions 逐段数值比较，缺段补 0（1.2 == 1.2.0）。
func compareVersions(a, b versionParts) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}
