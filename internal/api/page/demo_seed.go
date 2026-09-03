package page

// F：演示数据填充器（seed-demo）。
// 原则：一切走真实链路（Terms API 存储 → 生成器重算 → accept-and-publish
// 真实发布），不直接 INSERT 页面/提案，从而同时验证功能链路完整性。

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
)

type demoDataSeeder struct {
	svc    *Service
	gameID string
	env    string
}

func newDemoDataSeeder(svc *Service, gameID, env string) *demoDataSeeder {
	return &demoDataSeeder{svc: svc, gameID: gameID, env: env}
}

// demoTerms 演示词条：resource/operation key -> 显示名。
var demoTerms = []struct {
	Domain string
	Key    string
	Word   string
}{
	{"resource", "player", "玩家"},
	{"resource", "mail", "邮件"},
	{"resource", "order", "订单"},
	{"resource", "inventory", "背包"},
	{"operation", "ban", "封禁"},
	{"operation", "send", "发送"},
	{"operation", "list", "查询列表"},
	{"operation", "create", "创建"},
}

// demoWarnings 演示注册警告。
var demoWarnings = []struct {
	AgentID    string
	FunctionID string
	Code       string
	Message    string
}{
	{"agent-demo-1", "player.ban", "schema_optional_missing", "player.ban 缺少可选字段 reason 的描述，不影响注册"},
	{"agent-demo-2", "mail.send", "version_unchanged", "mail.send 版本号未随 schema 变更递增，建议 1.0.1"},
	{"agent-demo-1", "order.list", "low_call_rate", "order.list 近 24h 无调用，若已废弃请下线"},
}

func (s *demoDataSeeder) seed(ctx context.Context) (map[string]interface{}, error) {
	summary := map[string]interface{}{}

	// 1. Terms 词条
	termsWritten := 0
	for _, term := range demoTerms {
		if err := s.upsertTerm(ctx, term.Domain, term.Key, term.Word); err != nil {
			return nil, fmt.Errorf("upsert term %s/%s: %w", term.Domain, term.Key, err)
		}
		termsWritten++
	}
	summary["terms"] = termsWritten

	// 2. 重算提案（Terms 生效）+ 一键发布全部 ready/basic
	bulk, err := s.svc.BulkPublish(ctx, &PageBulkRequest{})
	if err != nil {
		return nil, err
	}
	summary["published"] = bulk.Published
	summary["publishFailed"] = bulk.Failed

	// 3. 演示注册警告（registry 内存存储，重启即失——演示用途）
	s.seedRegistrationWarnings()

	summary["registrationWarnings"] = len(demoWarnings)
	return summary, nil
}

func (s *demoDataSeeder) upsertTerm(ctx context.Context, domain, key, word string) error {
	term := &model.TermDictionary{
		Domain:  domain,
		TermKey: key,
		Alias:   key,
		Display: map[string]string{"zh-CN": word, "en-US": word},
	}
	return s.svc.svcCtx.TermDictModel.Upsert(ctx, term)
}

// seedRegistrationWarnings 通过 registry store 写入演示警告。
func (s *demoDataSeeder) seedRegistrationWarnings() {
	if s.svc.svcCtx.RegistryStore == nil {
		return
	}
	now := time.Now()
	for _, w := range demoWarnings {
		_ = s.svc.svcCtx.RegistryStore.UpsertRegistrationWarning(context.Background(), registry.FunctionRegistrationWarning{
			GameID:     s.gameID,
			Env:        s.env,
			AgentID:    w.AgentID,
			FunctionID: w.FunctionID,
			Version:    "1.0.0",
			Code:       w.Code,
			Message:    w.Message,
			FirstSeen:  now.Add(-2 * time.Hour),
			LastSeen:   now,
		})
	}
}
