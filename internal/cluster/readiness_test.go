package cluster

import (
	"errors"
	"testing"
	"time"
)

// TestReadiness_HealthyJoined 注册成功 + 续租成功 + 互联监听绑定 = 就绪。
func TestReadiness_HealthyJoined(t *testing.T) {
	r := NewReadiness(5*time.Second, 15*time.Second)
	r.SetJoined(true)
	r.SetInterconnect(true, nil)
	r.RenewOK()

	snap := r.Snapshot()
	if !snap.Joined || !snap.LeaseHealthy || !snap.InterconnectUp || !snap.Healthy {
		t.Fatalf("expected fully healthy, got %+v", snap)
	}
	if snap.Degraded != "" || snap.LeaseErr != "" || snap.InterconnectErr != "" {
		t.Fatalf("healthy snapshot must have no errors, got %+v", snap)
	}
}

// TestReadiness_LeaseWindow 租约健康窗口：连续失败 × 心跳 < 租约 TTL 仍
// 健康（单次抖动不摘流）；达到 TTL 窗口判租约不可用。
func TestReadiness_LeaseWindow(t *testing.T) {
	r := NewReadiness(5*time.Second, 15*time.Second)
	r.SetJoined(true)
	r.SetInterconnect(true, nil)

	// 2 次失败（10s < 15s）：仍健康（容忍单次抖动）。
	r.RenewFail(errors.New("conn refused"))
	r.RenewFail(errors.New("conn refused"))
	if !r.Snapshot().LeaseHealthy {
		t.Fatal("2 consecutive failures must stay lease-healthy (10s < 15s window)")
	}

	// 第 3 次失败（15s >= 15s）：租约判不可用。
	r.RenewFail(errors.New("conn refused"))
	snap := r.Snapshot()
	if snap.LeaseHealthy {
		t.Fatal("3 consecutive failures must mark lease unhealthy (window reached)")
	}
	if snap.Healthy {
		t.Fatal("unhealthy lease must make overall not healthy")
	}
	if snap.LeaseErr == "" {
		t.Fatal("lease error must be recorded")
	}

	// 恢复：续租成功重置计数。
	r.RenewOK()
	if !r.Snapshot().LeaseHealthy {
		t.Fatal("renew success must restore lease health")
	}
}

// TestReadiness_DegradeAndInterconnect 启动期降级与互联失败各自可观测。
func TestReadiness_DegradeAndInterconnect(t *testing.T) {
	r := NewReadiness(5*time.Second, 15*time.Second)

	// 启动期协调库不可用：未加入 + 降级原因。
	r.SetJoined(false)
	r.RecordDegrade("register: connection refused")
	snap := r.Snapshot()
	if snap.Joined || snap.Degraded == "" || snap.Healthy {
		t.Fatalf("degraded snapshot wrong: %+v", snap)
	}

	// 加入但互联监听失败：半加入态不可就绪。
	r2 := NewReadiness(5*time.Second, 15*time.Second)
	r2.SetJoined(true)
	r2.RenewOK()
	r2.SetInterconnect(false, errors.New("address in use"))
	snap2 := r2.Snapshot()
	if !snap2.Joined || snap2.InterconnectUp || snap2.Healthy {
		t.Fatalf("interconnect-down snapshot wrong: %+v", snap2)
	}
	if snap2.InterconnectErr == "" {
		t.Fatal("interconnect error must be recorded")
	}
}

// TestReadiness_NilSafe nil 就绪面方法恒安全（快照为零值）。
func TestReadiness_NilSafe(t *testing.T) {
	var r *Readiness
	r.SetJoined(true)
	r.RecordDegrade("x")
	r.RenewOK()
	r.RenewFail(errors.New("x"))
	r.SetInterconnect(false, nil)
	snap := r.Snapshot()
	if snap.Healthy || snap.Joined || snap.LeaseHealthy || snap.InterconnectUp {
		t.Fatalf("nil snapshot must be zero, got %+v", snap)
	}
}
