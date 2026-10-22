// heraldd 本地冒烟（herald 简档 §7 验收门「本地起 heraldd 冒烟，CI 不依赖
// 外网」）：起真 heraldd 后设 CROUPIER_HERALD_SMOKE_ADDR=<addr> 运行，验证
// HeraldOutlet 对真 herald 的品类注册/受理/dedup 折叠全链。未设环境变量
// 一律跳过（CI 与常规单测不依赖 herald 进程）。
//
// 本地复跑（herald 侧配置见 /tmp/herald-smoke.yaml 口径：log provider +
// groups 播种 + delivery.category_urgency 播种（缺省 normal 走 Inbox 面，
// 过不了 log 通道的 IM 强度）+ dedup.enabled（缺省关=闸门直通不折叠））：
//
//	cd ~/workspaces/herald && go build -o /tmp/heraldd ./cmd/heraldd
//	heraldd serve --config /tmp/herald-smoke.yaml &
//	CROUPIER_HERALD_SMOKE_ADDR=http://127.0.0.1:18090 \
//	  go test ./internal/platform/outlet/ -run TestHeralddSmoke -v
package outlet

import (
	"context"
	"os"
	"testing"
	"time"

	heraldsdk "github.com/cuihairu/herald/apps-sdk/go"
)

func TestHeralddSmoke(t *testing.T) {
	addr := os.Getenv("CROUPIER_HERALD_SMOKE_ADDR")
	if addr == "" {
		t.Skip("CROUPIER_HERALD_SMOKE_ADDR not set; local heraldd smoke skipped")
	}
	const token = "smoke-trigger-token"

	// 品类注册（一次性，走 herald 侧口径——冒烟里用 config face 直注）。
	admin := heraldsdk.New(addr, "croupier", token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := admin.RegisterCategory(ctx, HeraldCategoryAgentAlerts, "urgent"); err != nil {
		t.Fatalf("RegisterCategory(agent-alerts): %v", err)
	}
	if err := admin.RegisterCategory(ctx, HeraldCategoryAvailability, "urgent"); err != nil {
		t.Fatalf("RegisterCategory(availability): %v", err)
	}

	o := NewHeraldOutlet(addr, "croupier", token, "group:gm-ops")
	ev := sampleEvent()
	if _, err := o.Deliver(ctx, ev); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	// 同 EventID+DedupKey 重发：dedup 折叠（herald 侧幂等，受理成功不重投）。
	out, err := admin.Dispatch(ctx, heraldsdk.DispatchRequest{
		Category:  categoryForKind(ev.Kind),
		Urgency:   urgencyForSeverity(ev.Severity),
		Audiences: []string{HeraldDefaultTarget},
		EventID:   ev.EventID,
		DedupKey:  ev.DedupKey,
		Title:     ev.Title,
		Body:      ev.Body,
	})
	if err != nil {
		t.Fatalf("re-Dispatch: %v", err)
	}
	if !out.Suppressed {
		t.Fatalf("same EventID re-dispatch must be suppressed (dedup fold), got %+v", out)
	}
}
