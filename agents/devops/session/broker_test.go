package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/core/execlog"
)

// fakeAudit 捕获审计记录；admitErr 模拟审计通道不可写。
type fakeAudit struct {
	mu       sync.Mutex
	records  []execlog.Record
	admitErr error
}

func (f *fakeAudit) Admit(rec *execlog.Record) error {
	if f.admitErr != nil {
		return f.admitErr
	}
	f.mu.Lock()
	f.records = append(f.records, *rec)
	f.mu.Unlock()
	return nil
}

func (f *fakeAudit) Record(rec *execlog.Record) error {
	f.mu.Lock()
	f.records = append(f.records, *rec)
	f.mu.Unlock()
	return nil
}

func (f *fakeAudit) snapshot() []execlog.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]execlog.Record(nil), f.records...)
}

// stubTarget 捕获 Open 调用并回固定会话（或错误）。
type stubTarget struct {
	name    string
	opened  []SessionRequest
	session Session
	err     error
}

func (s *stubTarget) Name() string { return s.name }

func (s *stubTarget) Open(_ context.Context, req SessionRequest) (Session, error) {
	s.opened = append(s.opened, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.session, nil
}

type stubSession struct{ closed bool }

func (s *stubSession) ID() string { return "stub" }

func (s *stubSession) Close() error { s.closed = true; return nil }

func testBroker(t *testing.T, audit Audit, target Target) (*Broker, *Recorder) {
	t.Helper()
	rec, err := OpenRecorder(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("open recorder: %v", err)
	}
	t.Cleanup(func() { _ = rec.Close() })
	allow := NewAllowlist([]config.AllowHost{
		{Host: "10.0.1.20", Ports: []int{22}, Protocols: []string{"ssh"}},
	})
	b := NewBroker(allow, audit, target, rec, "devops", "devops-01", "demo", "prod")
	fixed := time.UnixMilli(1760000000000)
	b.now = func() time.Time { return fixed }
	return b, rec
}

func baseRequest() SessionRequest {
	return SessionRequest{
		Protocol: ProtocolSSH, Host: "10.0.1.20", Port: 22, Operator: "ops-甲",
	}
}

func TestBrokerHappyPathChain(t *testing.T) {
	audit := &fakeAudit{}
	target := &stubTarget{name: "stub", session: &stubSession{}}
	b, rec := testBroker(t, audit, target)

	sess, err := b.OpenSession(context.Background(), baseRequest())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(target.opened) != 1 {
		t.Fatalf("provider opened %d times, want 1", len(target.opened))
	}

	// admitted 放行凭据 + success 结果，顺序不可倒。
	if len(audit.records) != 2 {
		t.Fatalf("audit records = %d, want 2 (admitted+success)", len(audit.records))
	}
	if audit.records[0].Status != execlog.StatusAdmitted || audit.records[1].Status != execlog.StatusSuccess {
		t.Fatalf("statuses = %s/%s", audit.records[0].Status, audit.records[1].Status)
	}
	admit := audit.records[0]
	if admit.Operator != "ops-甲" || admit.Action != "session.open" ||
		admit.AgentType != "devops" || admit.AgentInstance != "devops-01" ||
		admit.GameID != "demo" || admit.Env != "prod" {
		t.Fatalf("admit record = %+v", admit)
	}
	if admit.TaskID == "" {
		t.Fatal("session id not assigned")
	}

	// 会话存档：started 行（Close 后再落 ended 行）。
	raw, err := os.ReadFile(rec.f.Name())
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if !strings.Contains(string(raw), `"event":"started"`) {
		t.Fatalf("archive missing started line: %s", raw)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	raw, _ = os.ReadFile(rec.f.Name())
	if !strings.Contains(string(raw), `"event":"ended"`) || !strings.Contains(string(raw), `"outcome":"closed"`) {
		t.Fatalf("archive missing ended line: %s", raw)
	}
}

func TestBrokerAllowlistRejectsBeforeProvider(t *testing.T) {
	audit := &fakeAudit{}
	target := &stubTarget{name: "stub", session: &stubSession{}}
	b, _ := testBroker(t, audit, target)

	// 主机不在名单。
	req := baseRequest()
	req.Host = "10.9.9.9"
	if _, err := b.OpenSession(context.Background(), req); err == nil {
		t.Fatal("non-allowlisted host accepted")
	}
	// 端口不在名单。
	req = baseRequest()
	req.Port = 3389
	if _, err := b.OpenSession(context.Background(), req); err == nil {
		t.Fatal("non-allowlisted port accepted")
	}
	// 协议不在名单。
	req = baseRequest()
	req.Protocol = ProtocolRDP
	if _, err := b.OpenSession(context.Background(), req); err == nil {
		t.Fatal("non-allowlisted protocol accepted")
	}
	if len(target.opened) != 0 {
		t.Fatalf("provider opened %d times on rejects, want 0", len(target.opened))
	}
	// 拒绝也留痕（谁试图进哪台），且无 admitted 凭据。
	for _, rec := range audit.records {
		if rec.Status == execlog.StatusAdmitted {
			t.Fatal("admitted credential issued on allowlist reject")
		}
	}
	if len(audit.records) != 3 {
		t.Fatalf("reject audit records = %d, want 3", len(audit.records))
	}
}

func TestBrokerAuditFirstRejectsWhenChannelDown(t *testing.T) {
	audit := &fakeAudit{admitErr: errors.New("disk full")}
	target := &stubTarget{name: "stub", session: &stubSession{}}
	b, _ := testBroker(t, audit, target)

	if _, err := b.OpenSession(context.Background(), baseRequest()); err == nil {
		t.Fatal("session opened without audit channel")
	}
	if len(target.opened) != 0 {
		t.Fatal("provider called despite audit-first rejection")
	}
}

func TestBrokerProviderFailureRecordsFailure(t *testing.T) {
	audit := &fakeAudit{}
	target := &stubTarget{name: "stub", err: errors.New("guacd unreachable")}
	b, _ := testBroker(t, audit, target)

	if _, err := b.OpenSession(context.Background(), baseRequest()); err == nil {
		t.Fatal("provider error swallowed")
	}
	if len(audit.records) != 2 {
		t.Fatalf("records = %d, want admitted+failure", len(audit.records))
	}
	last := audit.records[1]
	if last.Status != execlog.StatusFailure || !strings.Contains(last.Summary, "guacd unreachable") {
		t.Fatalf("failure record = %+v", last)
	}
}

func TestBrokerValidatesRequest(t *testing.T) {
	audit := &fakeAudit{}
	target := &stubTarget{name: "stub", session: &stubSession{}}
	b, _ := testBroker(t, audit, target)

	req := baseRequest()
	req.Operator = " "
	if _, err := b.OpenSession(context.Background(), req); err == nil {
		t.Fatal("blank operator accepted")
	}
	req = baseRequest()
	req.Protocol = Protocol("telnet")
	if _, err := b.OpenSession(context.Background(), req); err == nil {
		t.Fatal("out-of-set protocol accepted")
	}
	req = baseRequest()
	req.Host = ""
	if _, err := b.OpenSession(context.Background(), req); err == nil {
		t.Fatal("blank host accepted")
	}
	if len(target.opened) != 0 {
		t.Fatal("provider called on invalid requests")
	}
}

func TestAllowlistMatchSemantics(t *testing.T) {
	allow := NewAllowlist([]config.AllowHost{
		{Host: "Pack-01", Ports: []int{22}, Protocols: []string{"ssh"}},
	})
	if err := allow.Check("pack-01", 22, ProtocolSSH); err != nil {
		t.Fatalf("case-insensitive host match failed: %v", err)
	}
	if err := allow.Check("other", 22, ProtocolSSH); err == nil {
		t.Fatal("unknown host accepted")
	}
	if err := allow.Check("pack-01", 2222, ProtocolSSH); err == nil {
		t.Fatal("unlisted port accepted")
	}
	if err := allow.Check("pack-01", 22, ProtocolVNC); err == nil {
		t.Fatal("unlisted protocol accepted")
	}
}
