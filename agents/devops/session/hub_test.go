package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/core/execlog"
	"golang.org/x/crypto/ssh"
)

// 写测试文件并返回路径（独立临时目录）。
func writeTestFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// ---- SSH 通道（真实协议，进程内服务端）----

// newTestTarget 以真实 SSH 服务端装配 sshTarget（password 认证 + insecure
// host key 逃生门；known_hosts 校验链在专门用例覆盖）。
func newTestTarget(t *testing.T, user, pass string) Target {
	t.Helper()
	tgt, err := NewSSHChannelTarget(SSHConfig{
		User:            user,
		PasswordFile:    writeTestFile(t, "pass", []byte(pass+"\n")),
		InsecureHostKey: true,
	})
	if err != nil {
		t.Fatalf("new ssh target: %v", err)
	}
	return tgt
}

func TestSSHChannelOpenAndEcho(t *testing.T) {
	srv := newEchoSSHServer(t, "ops", "secret")
	tgt := newTestTarget(t, "ops", "secret")

	host, port := srv.HostPort()
	sess, err := tgt.Open(context.Background(), SessionRequest{
		SessionID: "sess-1", Protocol: ProtocolSSH, Host: host, Port: port,
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	term, ok := sess.(Terminal)
	if !ok {
		t.Fatal("ssh session does not implement Terminal")
	}
	if srv.PTYRequests() != 1 {
		t.Fatalf("pty requests = %d, want 1", srv.PTYRequests())
	}

	// 写入 → 服务端 echo → 读回。
	go func() {
		if _, err := term.Stdin().Write([]byte("hello devops\n")); err != nil {
			t.Errorf("stdin write: %v", err)
		}
	}()
	buf := make([]byte, 13)
	if _, err := io.ReadFull(term.Stdout(), buf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf) != "hello devops\n" {
		t.Fatalf("echo = %q", string(buf))
	}

	// Resize → 服务端收到 window-change（rows/cols 顺序以远端记录为准）。
	if err := term.Resize(120, 40); err != nil {
		t.Fatalf("resize: %v", err)
	}
	waitFor(t, func() bool { return len(srv.Resizes()) == 1 })
	if rz := srv.Resizes(); rz[0] != [2]int{120, 40} {
		t.Fatalf("resizes = %v, want [[120 40]]", rz)
	}

	// Close 幂等。
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestSSHChannelAuthRejected(t *testing.T) {
	srv := newEchoSSHServer(t, "ops", "secret")
	tgt := newTestTarget(t, "ops", "wrong")
	host, port := srv.HostPort()
	_, err := tgt.Open(context.Background(), SessionRequest{
		Protocol: ProtocolSSH, Host: host, Port: port,
	})
	if err == nil {
		t.Fatal("wrong password accepted")
	}
	if !strings.Contains(err.Error(), "handshake") {
		t.Fatalf("error = %v", err)
	}
}

func TestSSHChannelRejectsNonSSHProtocol(t *testing.T) {
	srv := newEchoSSHServer(t, "ops", "secret")
	tgt := newTestTarget(t, "ops", "secret")
	host, port := srv.HostPort()
	_, err := tgt.Open(context.Background(), SessionRequest{
		Protocol: ProtocolRDP, Host: host, Port: port,
	})
	if err == nil || !strings.Contains(err.Error(), "does not support protocol rdp") {
		t.Fatalf("rdp on ssh channel = %v", err)
	}
}

func TestSSHHostKeyVerification(t *testing.T) {
	srv := newEchoSSHServer(t, "ops", "secret")
	signer, err := ssh.ParsePrivateKey(testHostKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	authorized := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))

	// 1) 指纹匹配：knownhosts 链放行。
	tgtOK, err := NewSSHChannelTarget(SSHConfig{
		User:           "ops",
		PasswordFile:   writeTestFile(t, "pass", []byte("secret\n")),
		KnownHostsFile: writeTestFile(t, "kh_ok", []byte("[127.0.0.1]:"+itoa(srv.port())+" "+authorized+"\n")),
	})
	if err != nil {
		t.Fatalf("new target: %v", err)
	}
	host, port := srv.HostPort()
	sess, oerr := tgtOK.Open(context.Background(), SessionRequest{
		Protocol: ProtocolSSH, Host: host, Port: port,
	})
	if oerr != nil {
		t.Fatalf("known host key rejected: %v", oerr)
	}
	_ = sess.Close()

	// 2) 指纹不匹配（另一把 key）：必须拒。
	other, err := ssh.ParsePrivateKey(mustGenKeyPEM())
	if err != nil {
		t.Fatal(err)
	}
	otherAuthorized := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(other.PublicKey())))
	tgtBad, err := NewSSHChannelTarget(SSHConfig{
		User:           "ops",
		PasswordFile:   writeTestFile(t, "pass", []byte("secret\n")),
		KnownHostsFile: writeTestFile(t, "kh_bad", []byte("[127.0.0.1]:"+itoa(srv.port())+" "+otherAuthorized+"\n")),
	})
	if err != nil {
		t.Fatalf("new target: %v", err)
	}
	if _, oerr := tgtBad.Open(context.Background(), SessionRequest{
		Protocol: ProtocolSSH, Host: host, Port: port,
	}); oerr == nil {
		t.Fatal("unknown host key accepted")
	}
}

func TestSSHConfigValidation(t *testing.T) {
	if _, err := NewSSHChannelTarget(SSHConfig{}); err == nil {
		t.Fatal("empty config accepted")
	}
	if _, err := NewSSHChannelTarget(SSHConfig{User: "ops"}); err == nil {
		t.Fatal("no auth accepted")
	}
	if _, err := NewSSHChannelTarget(SSHConfig{User: "ops", PasswordFile: "x"}); err == nil {
		t.Fatal("no host key policy accepted")
	}
	if _, err := NewSSHChannelTarget(SSHConfig{
		User:            "ops",
		PasswordFile:    writeTestFile(t, "empty_pass", []byte("")),
		InsecureHostKey: true,
	}); err == nil {
		t.Fatal("empty password file accepted")
	}
	if _, err := NewSSHChannelTarget(SSHConfig{
		User:           "ops",
		PasswordFile:   writeTestFile(t, "pass", []byte("secret\n")),
		KnownHostsFile: filepath.Join(t.TempDir(), "missing_known_hosts"),
	}); err == nil {
		t.Fatal("missing known_hosts file accepted")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// ---- Hub（数据面）----

// fakeTerminal 可编程 Terminal（Hub 数据面单测；不依赖网络）。
type fakeTerminal struct {
	id      string
	stdinR  io.ReadCloser
	stdinW  io.WriteCloser
	stdoutR io.ReadCloser
	stdoutW io.WriteCloser
	resizes [][2]int
	mu      sync.Mutex
	closed  bool
}

func newFakeTerminal(id string) *fakeTerminal {
	sinR, sinW := io.Pipe()
	soutR, soutW := io.Pipe()
	serrR, serrW := io.Pipe()
	_ = serrR
	_ = serrW
	return &fakeTerminal{
		id: id, stdinR: sinR, stdinW: sinW,
		stdoutR: soutR, stdoutW: soutW,
	}
}

func (f *fakeTerminal) ID() string        { return f.id }
func (f *fakeTerminal) Stdin() io.Writer  { return f.stdinW }
func (f *fakeTerminal) Stdout() io.Reader { return f.stdoutR }
func (f *fakeTerminal) Stderr() io.Reader { return f.stdoutR } // 复用同流（测试足够）
func (f *fakeTerminal) Resize(c, r int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resizes = append(f.resizes, [2]int{c, r})
	return nil
}

// Wait 远端结束 = stdout EOF（Close 关写端触发）。
func (f *fakeTerminal) Wait() (TerminalExit, error) {
	buf := make([]byte, 64)
	for {
		if _, err := f.stdoutR.Read(buf); err != nil {
			return TerminalExit{ExitCode: 0}, nil
		}
	}
}

func (f *fakeTerminal) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	_ = f.stdinW.Close()
	_ = f.stdoutW.Close()
	_ = f.stdinR.Close()
	_ = f.stdoutR.Close()
	return nil
}

// fakeHubTarget 把预建 fakeTerminal 经 Target 语义交给 Broker。
type fakeHubTarget struct{ t Terminal }

func (f *fakeHubTarget) Name() string { return "fake" }
func (f *fakeHubTarget) Open(_ context.Context, req SessionRequest) (Session, error) {
	if req.Protocol != ProtocolSSH {
		return nil, errors.New("no channel")
	}
	return f.t, nil
}

type hubEnv struct {
	hub   *Hub
	audit *fakeAudit
	dir   string
}

func newHubEnv(t *testing.T, term Terminal, cfg HubConfig) *hubEnv {
	t.Helper()
	dir := t.TempDir()
	rec, err := OpenRecorder(filepath.Join(dir, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rec.Close() })
	audit := &fakeAudit{}
	allow := NewAllowlist([]config.AllowHost{
		{Host: "h1", Ports: []int{22}, Protocols: []string{"ssh"}},
	})
	broker := NewBroker(allow, audit, &fakeHubTarget{t: term}, rec, "devops", "d1", "demo", "prod")
	hub := NewHub(broker, rec, filepath.Join(dir, "casts"), cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &hubEnv{hub: hub, audit: audit, dir: dir}
}

func hubRequest() SessionRequest {
	return SessionRequest{Protocol: ProtocolSSH, Host: "h1", Port: 22, Operator: "ops-甲", Cols: 100, Rows: 30}
}

func TestHubOpenDataFlowAndCleanup(t *testing.T) {
	term := newFakeTerminal("s1")
	env := newHubEnv(t, term, HubConfig{InputWait: time.Second})
	ls, err := env.hub.Open(context.Background(), hubRequest())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if env.hub.Count() != 1 {
		t.Fatalf("live = %d", env.hub.Count())
	}

	var consumerBuf lockedBuffer
	detach, ok := ls.Attach(&consumerBuf)
	if !ok {
		t.Fatal("attach failed")
	}
	defer detach()

	if err := ls.Write([]byte("ls -la\r")); err != nil {
		t.Fatalf("write: %v", err)
	}
	// 模拟远端输出 → 消费者 + asciicast 双落。
	if _, err := term.stdoutW.Write([]byte("total 0\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return consumerBuf.String() == "total 0\n" })

	if err := ls.Resize(120, 40); err != nil {
		t.Fatalf("resize: %v", err)
	}
	term.mu.Lock()
	rz := append([][2]int(nil), term.resizes...)
	term.mu.Unlock()
	if len(rz) != 1 || rz[0] != [2]int{120, 40} {
		t.Fatalf("term resizes = %v", rz)
	}

	// 远端 EOF → 断线清理全链（end 审计行是清理末步，以它为收束信号）。
	_ = term.stdoutW.Close()
	waitFor(t, func() bool { return len(env.audit.snapshot()) >= 3 })

	// 审计：admitted + success + session.end（顺序不可倒）。
	records := env.audit.snapshot()
	if len(records) < 3 {
		t.Fatalf("audit records = %d", len(records))
	}
	if records[0].Status != execlog.StatusAdmitted ||
		records[1].Status != execlog.StatusSuccess {
		t.Fatalf("open chain = %s/%s", records[0].Status, records[1].Status)
	}
	last := records[len(records)-1]
	if last.Action != "session.end" || last.Status != execlog.StatusSuccess || last.Summary != "eof" {
		t.Fatalf("end record = %+v", last)
	}

	// 结束存档行带回放档统计。
	meta := readSessionsJSONL(t, filepath.Join(env.dir, "sessions", "sessions.jsonl"))
	var ended *SessionMeta
	for i := range meta {
		if meta[i].Event == "ended" {
			ended = &meta[i]
		}
	}
	if ended == nil {
		t.Fatal("no ended row")
	}
	if ended.Outcome != "eof" || ended.CastFile != "s1.cast" || ended.CastEvents == 0 || ended.CastBytes == 0 {
		t.Fatalf("ended row = %+v", ended)
	}

	// asciicast v2 可回放（形状校验：输出 + resize 事件都在）。
	events, err := ValidateCastFile(filepath.Join(env.dir, "casts", "s1.cast"))
	if err != nil {
		t.Fatalf("cast invalid: %v", err)
	}
	if events < 2 {
		t.Fatalf("cast events = %d, want >=2 (output + resize)", events)
	}

	// 会话收束后写/Resize 拒绝。
	if err := ls.Write([]byte("x")); err == nil {
		t.Fatal("write after close accepted")
	}
	if err := ls.Resize(80, 24); err == nil {
		t.Fatal("resize after close accepted")
	}
}

func TestHubBackpressureBusy(t *testing.T) {
	term := newFakeTerminal("s2")
	// stdin 无人读（pipe 写端阻塞）→ 泵卡死在消费第一段，队列必然填满：
	// 队列 2 + 泵在途 1，四段连写必有 busy（与泵启动时序无关的上界）。
	env := newHubEnv(t, term, HubConfig{InputQueue: 2, InputWait: 30 * time.Millisecond})
	ls, err := env.hub.Open(context.Background(), hubRequest())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	var busy bool
	for i := 0; i < 4; i++ {
		if err := ls.Write([]byte("x")); err != nil {
			if errors.Is(err, ErrSessionBusy) {
				busy = true
				break
			}
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if !busy {
		t.Fatal("unbounded input queue accepted (backpressure missing)")
	}
	_ = ls.Close()
	_ = env.hub.Close()
}

func TestHubConsumerEvictionOnTimeout(t *testing.T) {
	term := newFakeTerminal("s3")
	env := newHubEnv(t, term, HubConfig{AttachTimeout: 50 * time.Millisecond})
	ls, err := env.hub.Open(context.Background(), hubRequest())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = env.hub.Close() }()

	slow := &blockingWriter{release: make(chan struct{})}
	detach, ok := ls.Attach(slow)
	if !ok {
		t.Fatal("attach failed")
	}
	defer close(slow.release)

	if _, err := term.stdoutW.Write([]byte("out")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return ls.consumerCount() == 0 })
	_ = detach
	if !slow.writeStarted() {
		t.Fatal("consumer never received data")
	}

	// 摘除后新输出不再投递（不阻塞泵）。
	done := make(chan struct{})
	go func() {
		_, _ = term.stdoutW.Write([]byte("more"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pump blocked after consumer eviction")
	}
	_ = term.stdoutW.Close()
}

func TestHubMaxSessionsRefusal(t *testing.T) {
	term := newFakeTerminal("s4")
	env := newHubEnv(t, term, HubConfig{MaxSessions: 1})
	ls, err := env.hub.Open(context.Background(), hubRequest())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = env.hub.Close() }()
	_, err = env.hub.Open(context.Background(), hubRequest())
	if err == nil || !strings.Contains(err.Error(), "session limit") {
		t.Fatalf("over-limit open = %v", err)
	}
	// 拒绝留痕进审计（failure 行）。
	last := env.audit.snapshot()[len(env.audit.snapshot())-1]
	if last.Status != execlog.StatusFailure || !strings.Contains(last.Summary, "session limit") {
		t.Fatalf("refusal not audited: %+v", last)
	}
	_ = ls.Close()
}

func TestHubBadWindowRejected(t *testing.T) {
	term := newFakeTerminal("s5")
	env := newHubEnv(t, term, HubConfig{})
	req := hubRequest()
	req.Rows = 1001 // 越界（1..1000）
	if _, err := env.hub.Open(context.Background(), req); err == nil {
		t.Fatal("out-of-range window accepted")
	}
	_ = env.hub.Close()
}

func TestHubCloseAllIdempotent(t *testing.T) {
	term := newFakeTerminal("s6")
	env := newHubEnv(t, term, HubConfig{})
	ls, err := env.hub.Open(context.Background(), hubRequest())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := env.hub.Close(); err != nil {
		t.Fatalf("hub close: %v", err)
	}
	if env.hub.Count() != 0 {
		t.Fatalf("live after hub close = %d", env.hub.Count())
	}
	_ = ls.Close() // 幂等（cleanup 单点收敛）
}

// TestHubStdinBreakTriggersCleanup 回归：inputPump 致命条件（stdin 写失败）
// 原先直接调 cleanup → pumpWG.Wait 等本泵 → 死锁。修复后走 triggerCleanup
// 拆终端让 watch() 收束。验证不死锁、审计 outcome=error、存档完整。
func TestHubStdinBreakTriggersCleanup(t *testing.T) {
	term := newFakeTerminal("s7")
	env := newHubEnv(t, term, HubConfig{InputWait: time.Second})
	ls, err := env.hub.Open(context.Background(), hubRequest())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// 拆 stdin 写端——inputPump 写 stdin 必失败。
	_ = term.stdinW.Close()
	if err := ls.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	// watch() 经 triggerCleanup 收束 → 审计末行 session.end outcome=error。
	waitFor(t, func() bool { return len(env.audit.snapshot()) >= 3 })
	records := env.audit.snapshot()
	last := records[len(records)-1]
	if last.Action != "session.end" || last.Summary != "error" {
		t.Fatalf("end record = %+v", last)
	}
	if env.hub.Count() != 0 {
		t.Fatalf("live after stdin break = %d", env.hub.Count())
	}
}

// TestHubCastBrokenTriggersCleanup 回归：outputPump 存档断（磁盘满/权限）
// 原先直接调 cleanup → 死锁。修复后走 triggerCleanup（outcome=cast-broken）
// 让 watch() 收束。验证不死锁、审计 outcome=cast-broken。
func TestHubCastBrokenTriggersCleanup(t *testing.T) {
	term := newFakeTerminal("s8")
	env := newHubEnv(t, term, HubConfig{})
	ls, err := env.hub.Open(context.Background(), hubRequest())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// 直接置 cast sticky 错误（模拟磁盘满）——同包可见。
	ls.cast.mu.Lock()
	ls.cast.err = errors.New("disk full")
	ls.cast.mu.Unlock()
	// 产生输出 → outputPump 读到 → cast.Output no-op → cast.Err 非 nil →
	// triggerCleanup("cast-broken", ...)。
	if _, err := term.stdoutW.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	// watch() 收束 → 审计末行 outcome=cast-broken。
	waitFor(t, func() bool { return len(env.audit.snapshot()) >= 3 })
	records := env.audit.snapshot()
	last := records[len(records)-1]
	if last.Action != "session.end" || last.Summary != "cast-broken" {
		t.Fatalf("end record = %+v", last)
	}
	if env.hub.Count() != 0 {
		t.Fatalf("live after cast break = %d", env.hub.Count())
	}
}

func TestAllowlistSwapConcurrent(t *testing.T) {
	a := NewAllowlist([]config.AllowHost{
		{Host: "h1", Ports: []int{22}, Protocols: []string{"ssh"}},
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			a.Swap([]config.AllowHost{
				{Host: "h2", Ports: []int{2222}, Protocols: []string{"ssh"}},
				{Host: "h1", Ports: []int{22}, Protocols: []string{"ssh", "rdp"}},
			})
			a.Swap([]config.AllowHost{
				{Host: "h1", Ports: []int{22}, Protocols: []string{"ssh"}},
			})
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		// 读侧永远看到完整一致快照：h1:22 ssh 始终放行。
		if err := a.Check("h1", 22, ProtocolSSH); err != nil {
			t.Fatalf("snapshot torn: %v", err)
		}
		if n := a.Len(); n != 1 && n != 2 {
			t.Fatalf("len = %d", n)
		}
		if doneClosed(done) {
			break
		}
	}
	<-done
	prev, cur := a.Swap([]config.AllowHost{{Host: "h3", Ports: []int{80}, Protocols: []string{"rdp"}}})
	if prev != 1 || cur != 1 {
		t.Fatalf("swap counts = %d/%d", prev, cur)
	}
	if err := a.Check("h1", 22, ProtocolSSH); err == nil {
		t.Fatal("stale host still allowed after swap")
	}
}

func doneClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// ---- 测试小件 ----

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type blockingWriter struct {
	mu      sync.Mutex
	started bool
	release chan struct{}
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	if !w.started {
		w.started = true
		w.mu.Unlock()
		<-w.release // 第一次写阻塞到测试放行 → 触发消费超时摘除
		return len(p), nil
	}
	w.mu.Unlock()
	return len(p), nil
}

func (w *blockingWriter) writeStarted() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.started
}

// consumerCount 同包测试观察口：当前挂着的消费者数。
func (ls *LiveSession) consumerCount() int {
	ls.conMu.Lock()
	defer ls.conMu.Unlock()
	return len(ls.consumers)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within 3s")
}

func readSessionsJSONL(t *testing.T, path string) []SessionMeta {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []SessionMeta
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var m SessionMeta
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("bad jsonl: %v", err)
		}
		out = append(out, m)
	}
	return out
}
