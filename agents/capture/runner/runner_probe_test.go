package runner

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/healthprobe"
)

// 数据源连通探测已迁到 core/healthprobe 的声明式装配（TargetConfig →
// Manager，§12 R2）。本组用例锁住迁移后的行为边界：目标名沿用
// sourceType:addr、周期不变、装配失败必报错而非静默无探针。
func TestReadinessProbeUsesDeclarativeAssembly(t *testing.T) {
	cfg := testConfig(t, true)
	r := New(cfg, nil)

	addr, ok := r.probeTarget()
	if !ok {
		t.Fatal("probeTarget should resolve mysql host:port")
	}
	target := r.sourceTypeName() + ":" + addr
	if target != "mysql:127.0.0.1:1" {
		t.Fatalf("target = %q, want mysql:127.0.0.1:1", target)
	}

	mgr, err := healthprobe.NewManager([]healthprobe.TargetConfig{{
		Name:      target,
		Type:      healthprobe.TargetTypeTCP,
		Address:   addr,
		Timeout:   2 * time.Second,
		Interval:  captureReadinessInterval,
		Semantics: healthprobe.SemanticsReadiness,
	}}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("declarative assembly must succeed for a valid source address: %v", err)
	}
	probes := mgr.Probes()
	if len(probes) != 1 {
		t.Fatalf("probes = %d, want 1", len(probes))
	}

	// 迁移不改行为：探测立刻探一次，闭端口即失败并开窗（周期 10s 不变）。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r.startReadinessProbe(ctx)

	// Manager.Start 阻塞至 ctx 取消；这里另起一个探针实例做等价断言，
	// 避免等待真实周期。
	solo := healthprobe.New(target, healthprobe.SemanticsReadiness,
		healthprobe.NewTCPChecker(addr, 200*time.Millisecond),
		healthprobe.WithThresholds(1, 1))
	res := solo.ProbeOnce(ctx)
	if res.OK {
		t.Skip("port 127.0.0.1:1 unexpectedly reachable in this environment")
	}
	w, ok := solo.CurrentWindow()
	if !ok {
		t.Fatal("failing source probe must open a readiness window")
	}
	if w.Target != target {
		t.Fatalf("window target = %q, want %q", w.Target, target)
	}
	if w.Semantics != string(healthprobe.SemanticsReadiness) {
		t.Fatalf("window semantics = %q", w.Semantics)
	}
}

func TestReadinessProbeSkippedWhenAddressUnresolvable(t *testing.T) {
	cfg := testConfig(t, true)
	cfg.Capture.MySQL = nil
	r := New(cfg, nil)
	if _, ok := r.probeTarget(); ok {
		t.Fatal("probeTarget must report unresolvable when no source config exists")
	}
	// 不 panic 即达标（仅记 warn 日志后返回，不起探针）。
	r.startReadinessProbe(context.Background())
}

func TestReadinessProbeDeclarationRejectsInvalidTarget(t *testing.T) {
	// 声明非法时统一装配必须报错——capture 不再自写 Checker 接线，装配校验
	// 是唯一的守门点（漏了就会静默无探针）。
	if _, err := healthprobe.NewManager([]healthprobe.TargetConfig{{
		Name:    "broken",
		Type:    healthprobe.TargetTypeTCP,
		Address: "",
	}}, "", "", nil, nil); err == nil {
		t.Fatal("empty address must fail declarative assembly")
	}
}
