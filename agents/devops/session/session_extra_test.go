package session

// 补测：ExecAudit execlog 适配（真实 Gate+Logger 落盘）、Hub.Get/ID/Meta、
// Broker 白名单热替换与通道名、Recorder 守卫与关闭态、OpenCast 守卫、
// ValidateCastFile 全坏形分支。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/core/execlog"
	"github.com/stretchr/testify/require"
)

func TestExecAuditAdapterRealChain(t *testing.T) {
	w, err := execlog.OpenFileWriter(t.TempDir(), "exec.jsonl", execlog.WriterOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	logger := execlog.New(w)
	ea := &ExecAudit{Gate: logger.Gate(), Log: logger}

	rec := &execlog.Record{Operator: "ops-甲", Action: actionSessionOpen, GameID: "demo", Env: "prod"}
	require.NoError(t, ea.Admit(rec))
	require.Equal(t, execlog.StatusAdmitted, rec.Status)
	rec.Action = actionSessionEnd
	require.NoError(t, ea.Record(rec))
}

func TestBrokerSwapAllowlistAndChannel(t *testing.T) {
	audit := &fakeAudit{}
	allow := NewAllowlist([]config.AllowHost{
		{Host: "h1", Ports: []int{22}, Protocols: []string{"ssh"}},
	})
	target := &stubTarget{name: "stub-ssh", session: &stubSession{}}
	broker := NewBroker(allow, audit, target, nil, "devops", "d1", "demo", "prod")

	require.Equal(t, "stub-ssh", broker.Channel())
	var nilBroker *Broker
	require.Empty(t, nilBroker.Channel())

	// 热替换：1 → 2 条，新主机即放行
	prev, cur := broker.SwapAllowlist([]config.AllowHost{
		{Host: "h1", Ports: []int{22}, Protocols: []string{"ssh"}},
		{Host: "h2", Ports: []int{2222}, Protocols: []string{"ssh"}},
	})
	require.Equal(t, 1, prev)
	require.Equal(t, 2, cur)
	sess, err := broker.OpenSession(context.Background(), SessionRequest{
		Protocol: ProtocolSSH, Host: "h2", Port: 2222, Operator: "ops",
	})
	require.NoError(t, err)
	require.NoError(t, sess.Close())

	// 换成空名单 → 原主机也拒
	prev, cur = broker.SwapAllowlist(nil)
	require.Equal(t, 2, prev)
	require.Equal(t, 0, cur)
	_, err = broker.OpenSession(context.Background(), SessionRequest{
		Protocol: ProtocolSSH, Host: "h1", Port: 22, Operator: "ops",
	})
	require.Error(t, err)
}

func TestHubGetIDMeta(t *testing.T) {
	term := newFakeTerminal("s1")
	env := newHubEnv(t, term, HubConfig{InputWait: time.Second})
	ls, err := env.hub.Open(context.Background(), hubRequest())
	require.NoError(t, err)

	got, ok := env.hub.Get(ls.ID())
	require.True(t, ok)
	require.Same(t, ls, got)
	_, ok = env.hub.Get("no-such-session")
	require.False(t, ok)

	meta := ls.Meta()
	require.Equal(t, "h1", meta.Host)
	require.Equal(t, 22, meta.Port)
	require.Equal(t, "ops-甲", meta.Operator)
	require.Equal(t, ProtocolSSH, Protocol(meta.Protocol))
	require.NotEmpty(t, meta.SessionID)
	require.NotZero(t, meta.StartedUnixMs)

	require.NoError(t, ls.Close())
	_, ok = env.hub.Get(ls.ID())
	require.False(t, ok, "closed session must be deregistered")
}

func TestOpenRecorderGuardsAndClosedState(t *testing.T) {
	_, err := OpenRecorder("")
	require.ErrorContains(t, err, "recorder dir is required")

	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))
	_, err = OpenRecorder(filepath.Join(blocker, "sub"))
	require.Error(t, err)

	rec, err := OpenRecorder(filepath.Join(t.TempDir(), "sessions"))
	require.NoError(t, err)
	require.NoError(t, rec.Start(SessionMeta{SessionID: "s1", Channel: "ssh"}))
	require.NoError(t, rec.End("s1", "eof"))
	require.NoError(t, rec.Close())

	var nilRec *Recorder
	require.ErrorContains(t, nilRec.Start(SessionMeta{SessionID: "x"}), "recorder is closed")
	require.ErrorContains(t, nilRec.End("x", "eof"), "recorder is closed")
	require.ErrorContains(t, nilRec.EndStats("x", "eof", CastStats{}), "recorder is closed")
}

func TestOpenCastGuards(t *testing.T) {
	dir := t.TempDir()
	header := CastHeader{Version: 0}

	_, err := OpenCast("", "s1", header, time.Unix(0, 0))
	require.ErrorContains(t, err, "cast dir is required")
	_, err = OpenCast(dir, "", header, time.Unix(0, 0))
	require.ErrorContains(t, err, "cast session id is required")
	_, err = OpenCast(dir, "../evil", header, time.Unix(0, 0))
	require.ErrorContains(t, err, "invalid characters")
	_, err = OpenCast(dir, "a/b", header, time.Unix(0, 0))
	require.ErrorContains(t, err, "invalid characters")

	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))
	_, err = OpenCast(filepath.Join(blocker, "sub"), "s1", header, time.Unix(0, 0))
	require.Error(t, err)

	// Version=0 归一为 v2；正常打开可写
	w, err := OpenCast(filepath.Join(dir, "casts"), "s1", header, time.Unix(0, 0))
	require.NoError(t, err)
	w.Output([]byte("hi"))
	require.NoError(t, w.Close())
}

func TestValidateCastFile_BadShapes(t *testing.T) {
	write := func(name, content string) string {
		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		return path
	}

	// 文件缺失
	_, err := ValidateCastFile(filepath.Join(t.TempDir(), "missing.cast"))
	require.Error(t, err)
	// 只有 header 无事件
	_, err = ValidateCastFile(write("header-only", "{\"version\":2}\n"))
	require.ErrorContains(t, err, "at least one event")
	// header 损坏 / 版本不对
	_, err = ValidateCastFile(write("bad-header", "{bad\n{\"time\":0}\n"))
	require.ErrorContains(t, err, "header")
	_, err = ValidateCastFile(write("bad-version", "{\"version\":3}\n{\"time\":0}\n"))
	require.ErrorContains(t, err, "want 2")
	// 事件行形态各分支
	_, err = ValidateCastFile(write("bad-event", "{\"version\":2}\n{bad\n"))
	require.ErrorContains(t, err, "event 0")
	_, err = ValidateCastFile(write("neg-time", "{\"version\":2}\n{\"time\":-1,\"event\":\"o\",\"eventdata\":\"x\"}\n"))
	require.ErrorContains(t, err, "negative time")
	_, err = ValidateCastFile(write("bad-o", "{\"version\":2}\n{\"time\":0,\"event\":\"o\",\"eventdata\":5}\n"))
	require.ErrorContains(t, err, "output data not a string")
	_, err = ValidateCastFile(write("bad-r", "{\"version\":2}\n{\"time\":0,\"event\":\"r\",\"eventdata\":[1]}\n"))
	require.ErrorContains(t, err, "resize data not [cols,rows]")
	_, err = ValidateCastFile(write("bad-type", "{\"version\":2}\n{\"time\":0,\"event\":\"x\",\"eventdata\":\"x\"}\n"))
	require.ErrorContains(t, err, "unknown event type")

	// 合法档（header + 输出 + resize）
	_, err = ValidateCastFile(write("good",
		`{"version":2}`+"\n"+`{"time":0,"event":"o","eventdata":"hi"}`+"\n"+`{"time":1,"event":"r","eventdata":[120,40]}`+"\n"))
	require.NoError(t, err)
}

func TestCastFileNameWhitelist(t *testing.T) {
	require.Equal(t, "s1.cast", castFileName("s1"))
	require.True(t, strings.HasSuffix(castFileName("abc-DEF_09"), ".cast"))
}
