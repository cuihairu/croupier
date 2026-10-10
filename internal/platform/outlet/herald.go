// herald 出口：Outlet 接口的第一内置实现。croupier 是「事件语义在自
// 己词汇里的整合方」，经 herald apps-sdk/go 的 trigger Client 投递
// （不自研 REST 封装）；courier 式接法——croupier 不配渠道不配收件人，
// Audiences 恒为配置的默认受众 ref（herald 侧解析，见 herald 对接简档
// §3/§4）。herald Error（拒绝：凭据/契约，重试无意义）与 Transport 错
// 误（网络层，可重试）分别分类。
package outlet

import (
	"context"
	"strings"

	heraldsdk "github.com/cuihairu/herald/apps-sdk/go"
)

// herald 品类（herald 简档 §2 品类表；herald 侧须先注册品类，courier
// 式播种走 herald 配置文件，不在 croupier 侧）。
const (
	HeraldCategoryAgentAlerts  = "agent-alerts" // capture/devops/supervisor 告警
	HeraldCategoryAvailability = "availability" // healthprobe 窗口
)

// HeraldDefaultTarget 是 v1 固定默认受众 ref（简档 §3：v1 不逐事件指定
// 受众，由品类默认路由——herald dispatch face 要求至少一个受众 ref，故
// 配置必填；herald 侧解析 ref 到群组/值班表）。
const HeraldDefaultTarget = "group:gm-ops"

// 可选配置字段的内置默认值（configs/server.yaml `herald:` 段留空时生效；
// tokenEnv 默认指 HERALD_TRIGGER_TOKEN——凭证走环境变量引用，不落配置文件）。
const (
	HeraldDefaultApp      = "croupier"
	HeraldDefaultTokenEnv = "HERALD_TRIGGER_TOKEN"
)

// categoryForKind 把 kind 闭集映射到 herald 品类（简档 §2 品类表，
// 前缀映射；未知 kind 兜底 agent-alerts——closed set 之外是调用方 bug，
// 不静默造新品类）。
func categoryForKind(kind string) string {
	if strings.HasPrefix(kind, "probe.") {
		return HeraldCategoryAvailability
	}
	return HeraldCategoryAgentAlerts
}

// urgencyForSeverity 把 croupier 三档严重度映射为 herald 紧急度（herald
// 事件适配面同款映射：critical→critical，warning→urgent，info→normal）。
func urgencyForSeverity(severity string) string {
	switch severity {
	case SeverityCritical:
		return "critical"
	case SeverityWarning:
		return "urgent"
	default:
		return "normal"
	}
}

// HeraldOutlet 经 herald apps-sdk Dispatch（trigger face）投递信封。
type HeraldOutlet struct {
	client *heraldsdk.Client
	target string
}

// NewHeraldOutlet builds the outlet for one herald namespace。app/target
// 传空取内置默认；baseURL 与 token 由调用方判空（enabled 部署缺一即不
// 注册出口，svc 布线负责）。
func NewHeraldOutlet(baseURL, app, token, target string) *HeraldOutlet {
	if app == "" {
		app = HeraldDefaultApp
	}
	if target == "" {
		target = HeraldDefaultTarget
	}
	return &HeraldOutlet{
		client: heraldsdk.New(baseURL, app, token),
		target: target,
	}
}

// Name implements Outlet.
func (h *HeraldOutlet) Name() string { return "herald" }

// Deliver maps the envelope onto one dispatch call. 拒绝类错误（herald
// Error）包 Permanent——重试无意义；Transport 错误原样返回由管理器重试。
// ev.Target 非空时覆盖默认受众（如 category-leader:<slug>——报表分片
// 按类别 leader 定向，incident-reports §6）。正文尾随 gate 标注
// （source_unknown 等——herald DispatchRequest 无自由 metadata 位，
// 尾注是标注到达值班人的最小面）。
func (h *HeraldOutlet) Deliver(ctx context.Context, ev AlertEvent) (DeliveryOutcome, error) {
	category := categoryForKind(ev.Kind)
	audience := h.target
	if ev.Target != "" {
		audience = ev.Target
	}
	_, err := h.client.Dispatch(ctx, heraldsdk.DispatchRequest{
		Category:  category,
		Urgency:   urgencyForSeverity(ev.Severity),
		Audiences: []string{audience},
		DedupKey:  ev.DedupKey,
		EventID:   ev.EventID,
		Title:     ev.Title,
		Body:      ev.Body + metadataSuffix(ev.Metadata),
	})
	if err != nil {
		if heraldsdk.IsTransport(err) {
			return DeliveryOutcome{Delivered: false, Channel: category, Retryable: true}, err
		}
		return DeliveryOutcome{Delivered: false, Channel: category, Retryable: false}, Permanent(err)
	}
	return DeliveryOutcome{Delivered: true, Channel: category}, nil
}
