package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"time"
)

// HubConfig 会话面数据面参数（背压与上限全部显式、全部有界——无界缓冲即
// 无界内存，堡垒机会话面不接受）。
type HubConfig struct {
	// MaxSessions 并发会话上限（0 = 16）。超出即拒（session limit reached），
	// 拒绝留痕走 Broker.Refuse。
	MaxSessions int
	// InputQueue 终端输入有界队列长度（0 = 256 段）。
	InputQueue int
	// InputWait 队列满载时调用方最大等待（0 = 2s；超时回 ErrSessionBusy，
	// 键盘数据不静默丢弃——调用方退避重试）。
	InputWait time.Duration
	// AttachTimeout 消费者单次写超时（0 = 2s；超时摘除慢消费者，会话与
	// 存档不受影响）。
	AttachTimeout time.Duration
	// CastChunk 单条 asciicast 输出事件最大字节（0 = 8KiB；超长切条，
	// 回放等价、单行内存有界）。
	CastChunk int
}

// defaults 填缺省（显式复制，不改入参）。
func (c HubConfig) withDefaults() HubConfig {
	if c.MaxSessions <= 0 {
		c.MaxSessions = 16
	}
	if c.InputQueue <= 0 {
		c.InputQueue = 256
	}
	if c.InputWait <= 0 {
		c.InputWait = 2 * time.Second
	}
	if c.AttachTimeout <= 0 {
		c.AttachTimeout = 2 * time.Second
	}
	if c.CastChunk <= 0 {
		c.CastChunk = 8 * 1024
	}
	return c
}

// Hub 会话面数据面：建立链（Broker，顺序不可倒）之上挂活跃会话登记、
// 双向数据流、窗口调整、内容回放存档与断线清理。
//
//   - 背压三面：并发上限（准入拒）、输入有界队列+超时（ErrSessionBusy）、
//     消费者写超时摘除（慢消费者不拖死会话与泵）。
//   - 断线清理单一收束点 cleanup（closeOnce）：远端 EOF / 本地 Close /
//     Hub.Close / 存档断 任一先到即触发，幂等——hub 摘除、输入队列关闭、
//     终端断链、asciicast 收尾、结束存档与会话结束审计全走这里。
type Hub struct {
	broker  *Broker
	rec     *Recorder // nil = 不落会话起止元档案（审计仍有 execlog 真值）
	castDir string
	cfg     HubConfig
	log     *slog.Logger
	now     func() time.Time

	mu   sync.Mutex
	live map[string]*LiveSession
}

// NewHub 装配会话面。broker 走完整建立链（白名单→审计前置→Open）；castDir
// 为会话内容回放存档目录；rec 为起止元档案（可 nil = 只审计不留档）。
func NewHub(broker *Broker, rec *Recorder, castDir string, cfg HubConfig, log *slog.Logger) *Hub {
	if log == nil {
		log = slog.Default()
	}
	return &Hub{
		broker:  broker,
		castDir: castDir,
		cfg:     cfg.withDefaults(),
		log:     log,
		now:     time.Now,
		live:    make(map[string]*LiveSession),
		rec:     rec,
	}
}

// Open 走完整建立链并接入数据面：准入上限 → Broker（校验→白名单→审计前置
// →通道 Open）→ 内容存档打开 → 数据泵启动。存档打不开 = 回放留痕断，拒开
// 会话（审计已记 success 由 Refuse 语义补记结束行，见 cast 失败分支注释）。
func (h *Hub) Open(ctx context.Context, req SessionRequest) (*LiveSession, error) {
	if h.broker == nil {
		return nil, errors.New("session: hub broker not wired")
	}
	h.mu.Lock()
	if len(h.live) >= h.cfg.MaxSessions {
		h.mu.Unlock()
		h.broker.Refuse(req, "session limit reached")
		return nil, errors.New("session: session limit reached")
	}
	h.mu.Unlock()

	sess, err := h.broker.OpenSession(ctx, req)
	if err != nil {
		return nil, err
	}
	// 会话 id 以建立链产出为准（Broker 为空请求生成）。
	id := sess.ID()
	term, ok := sess.(Terminal)
	if !ok {
		// 通道未实现终端面（如 guacd 批次前的占位）——建立链已放行，这里
		// 只能拒数据面并补记结束行，不能伪装成可交互会话。
		_ = sess.Close()
		h.broker.RecordEnd(id, req.Operator, "no-terminal-channel", 0)
		return nil, fmt.Errorf("session: channel %s has no terminal surface", h.broker.Channel())
	}

	// 内容回放存档（audit-first 的内容面：打不开 = 回放断，拒开）。
	cols, rows := req.Cols, req.Rows
	if cols <= 0 {
		cols = DefaultCols
	}
	if rows <= 0 {
		rows = DefaultRows
	}
	if err := ValidateWindow(cols, rows); err != nil {
		_ = sess.Close()
		h.broker.RecordEnd(id, req.Operator, "bad-window", 0)
		return nil, err
	}
	started := h.now()
	cast, err := OpenCast(h.castDir, id, CastHeader{
		Version:   2,
		Width:     cols,
		Height:    rows,
		Timestamp: started.Unix(),
		Title:     fmt.Sprintf("%s@%s:%d", req.Operator, req.Host, req.Port),
		Env:       map[string]string{"SHELL": "/bin/sh", "TERM": sshTerm},
	}, started)
	if err != nil {
		_ = sess.Close()
		h.broker.RecordEnd(id, req.Operator, "cast-open-failed", 0)
		return nil, err
	}

	ls := &LiveSession{
		hub: h,
		meta: SessionMeta{
			SessionID:     id,
			Protocol:      string(req.Protocol),
			Host:          req.Host,
			Port:          req.Port,
			Operator:      req.Operator,
			Channel:       h.broker.Channel(),
			StartedUnixMs: started.UnixMilli(),
		},
		term:      term,
		cast:      cast,
		started:   started,
		inputCh:   make(chan []byte, h.cfg.InputQueue),
		done:      make(chan struct{}),
		consumers: make(map[int64]*consumer),
	}
	h.mu.Lock()
	h.live[id] = ls
	h.mu.Unlock()

	ls.startPumps()
	go ls.watch()
	h.log.Info("devops session opened", "sessionId", id,
		"operator", req.Operator, "target", fmt.Sprintf("%s:%d", req.Host, req.Port),
		"channel", ls.meta.Channel, "window", fmt.Sprintf("%dx%d", cols, rows))
	return ls, nil
}

// Get 取活跃会话。
func (h *Hub) Get(sessionID string) (*LiveSession, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ls, ok := h.live[sessionID]
	return ls, ok
}

// Count 活跃会话数。
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.live)
}

// Close 收束全部活跃会话（runner 停机收尾）。
func (h *Hub) Close() error {
	h.mu.Lock()
	sessions := make([]*LiveSession, 0, len(h.live))
	for _, ls := range h.live {
		sessions = append(sessions, ls)
	}
	h.mu.Unlock()
	var firstErr error
	for _, ls := range sessions {
		if err := ls.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// remove 摘除登记（cleanup 单点调用）。
func (h *Hub) remove(ls *LiveSession) {
	h.mu.Lock()
	delete(h.live, ls.meta.SessionID)
	h.mu.Unlock()
}

// LiveSession 一条活跃交互会话：SSH 三向数据流 + 回放存档 + 消费者扇出。
type LiveSession struct {
	hub     *Hub
	meta    SessionMeta
	term    Terminal
	cast    *CastWriter
	started time.Time

	inputCh   chan []byte
	pumpWG    sync.WaitGroup
	done      chan struct{}
	closeOnce onceErr

	// pendOutcome/pendCause 由泵在致命条件下经 triggerCleanup 记录
	// （cast-broken / stdin 断）。watch() 收束时若泵已置位则优先采用泵的
	// outcome——泵不能直接调 cleanup（cleanup 内 pumpWG.Wait 会等调方泵，
	// 死锁），只拆终端让 watch() 收束。
	pendMu      sync.Mutex
	pendOutcome string
	pendCause   error
	pendSet     bool

	conMu     sync.Mutex
	consumers map[int64]*consumer
	nextConID int64
}

// consumer 数据面消费者（面板/中继侧的写出口）。
type consumer struct {
	id      int64
	w       io.Writer
	timeout time.Duration
	mu      sync.Mutex
	evicted bool
}

// ID 会话 id。
func (ls *LiveSession) ID() string { return ls.meta.SessionID }

// Meta 会话起止元数据（started 态快照）。
func (ls *LiveSession) Meta() SessionMeta { return ls.meta }

// Write 终端输入（键盘数据）。有界队列+有限等待：满载超时回 ErrSessionBusy
// （调用方退避重试；键盘数据不静默丢弃）。会话已收束则拒写。
func (ls *LiveSession) Write(p []byte) error {
	select {
	case <-ls.done:
		return errors.New("session: session closed")
	default:
	}
	if len(p) == 0 {
		return nil
	}
	buf := make([]byte, len(p))
	copy(buf, p)
	select {
	case ls.inputCh <- buf:
		return nil
	case <-ls.done:
		return errors.New("session: session closed")
	case <-time.After(ls.hub.cfg.InputWait):
		return ErrSessionBusy
	}
}

// Resize 调整远端 pty 窗口并落存档事件（回放跟随）。
func (ls *LiveSession) Resize(cols, rows int) error {
	select {
	case <-ls.done:
		return errors.New("session: session closed")
	default:
	}
	if err := ls.term.Resize(cols, rows); err != nil {
		return err
	}
	ls.cast.Resize(cols, rows)
	return nil
}

// Attach 挂数据面消费者（后续批次 server 中继/面板接线的接线点；本批测试
// 与本地消费直接用）。返回 detach（幂等）。写超时的消费者被摘除（返回值
// false），会话与存档不受影响。
func (ls *LiveSession) Attach(w io.Writer) (detach func(), ok bool) {
	ls.conMu.Lock()
	defer ls.conMu.Unlock()
	select {
	case <-ls.done:
		return func() {}, false
	default:
	}
	c := &consumer{id: ls.nextConID, w: w, timeout: ls.hub.cfg.AttachTimeout}
	ls.nextConID++
	ls.consumers[c.id] = c
	id := c.id
	return func() {
		ls.conMu.Lock()
		defer ls.conMu.Unlock()
		if c, exist := ls.consumers[id]; exist {
			c.evicted = true
			delete(ls.consumers, id)
		}
	}, true
}

// CastPath 回放存档路径（运维抽查/上传）。
func (ls *LiveSession) CastPath() string {
	return filepath.Join(ls.hub.castDir, castFileName(ls.meta.SessionID))
}

// CastStats 存档收尾统计（结束存档行携带）。
type CastStats struct {
	File   string
	Events int
	Bytes  int64
	Err    error
}

// Close 主动断开（操作者/停机；幂等）。触发断线清理全链。
func (ls *LiveSession) Close() error {
	return ls.cleanup("closed", nil)
}

// watch 断线监视：远端会话结束（EOF/退出/错误）即收束。泵在致命条件下
// （cast 断 / stdin 断）不直接调 cleanup——cleanup 内 pumpWG.Wait 会等调方
// 泵，死锁。泵改调 triggerCleanup 拆终端让本函数收束，并预置 outcome。
func (ls *LiveSession) watch() {
	exit, err := ls.term.Wait()
	outcome := "eof"
	if err != nil {
		outcome = "error"
	}
	cause := err
	// 泵预置的 outcome 优先（cast-broken / stdin 断比 Wait 的 eof 更准确）。
	ls.pendMu.Lock()
	if ls.pendSet {
		outcome = ls.pendOutcome
		cause = ls.pendCause
	}
	ls.pendMu.Unlock()
	_ = exit
	_ = ls.cleanup(outcome, cause)
}

// triggerCleanup 泵致命条件收束入口（不死锁）：记录 outcome/cause → 关终端
// （拆链让其他泵 EOF 收尾、Wait 返回唤醒 watch）→ 立即返回。绝不调 cleanup
// （cleanup 内 pumpWG.Wait 会等调方泵）。watch() 或 Close() 走 cleanup。
func (ls *LiveSession) triggerCleanup(outcome string, cause error) {
	ls.pendMu.Lock()
	if !ls.pendSet {
		ls.pendOutcome = outcome
		ls.pendCause = cause
		ls.pendSet = true
	}
	ls.pendMu.Unlock()
	_ = ls.term.Close()
}

// cleanup 断线清理单一收束点（幂等）：摘登记 → 关终端/队列 → 泵排空 →
// 存档收尾 → 结束存档行 → 会话结束审计。outcome ∈ closed|eof|error|
// cast-broken。
func (ls *LiveSession) cleanup(outcome string, cause error) error {
	return ls.closeOnce.Do(func() error {
		ls.hub.remove(ls)
		// 先关终端：pipe 拆链让输入泵与输出泵立即 EOF 收尾。
		closeErr := ls.term.Close()
		close(ls.inputCh) // 输入泵排空残量后退出
		ls.pumpWG.Wait()
		close(ls.done)
		// 摘除全部消费者（迟到的 fan-out 写不再进入）。
		ls.conMu.Lock()
		for id, c := range ls.consumers {
			c.evicted = true
			delete(ls.consumers, id)
		}
		ls.conMu.Unlock()

		castErr := ls.cast.Close()
		final := outcome
		if final == "eof" && castErr != nil {
			// 存档断 = 回放不完整，结束行如实标注。
			final = "cast-broken"
		}
		if ls.hub.rec != nil {
			_ = ls.hub.rec.EndStats(ls.meta.SessionID, final, CastStats{
				File:   ls.CastPath(),
				Events: ls.cast.Events(),
				Bytes:  ls.cast.written(),
			})
		}
		elapsed := ls.hub.now().Sub(ls.started).Milliseconds()
		ls.hub.broker.RecordEnd(ls.meta.SessionID, ls.meta.Operator, final, elapsed)
		ls.hub.log.Info("devops session ended", "sessionId", ls.meta.SessionID,
			"operator", ls.meta.Operator, "outcome", final,
			"durationMs", elapsed, "castEvents", ls.cast.Events(),
			"castErr", castErrString(castErr), "cause", errorString(cause), "closeErr", errorString(closeErr))
		return closeErr
	})
}

func castErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// startPumps 启动输出双泵（stdout/stderr）与输入泵。
func (ls *LiveSession) startPumps() {
	ls.pumpWG.Add(3)
	go ls.outputPump("stdout", ls.term.Stdout())
	go ls.outputPump("stderr", ls.term.Stderr())
	go ls.inputPump()
}

// outputPump 单向输出泵：终端 → asciicast 事件 + 消费者扇出。cast 落盘
// 同步（存档完整性优先）；cast 断 = 回放断，立即收束会话（不无档续跑）。
func (ls *LiveSession) outputPump(name string, r io.Reader) {
	defer ls.pumpWG.Done()
	chunk := ls.hub.cfg.CastChunk
	buf := make([]byte, chunk)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			ls.cast.Output(buf[:n])
			if ls.cast.Err() != nil {
				// 存档断（磁盘满/权限）：拆终端让 watch() 以 cast-broken 收束。
				// 不直接调 cleanup——cleanup 内 pumpWG.Wait 会等本泵，死锁。
				ls.triggerCleanup("cast-broken", ls.cast.Err())
				return
			}
			ls.fanOut(buf[:n])
		}
		if err != nil {
			if err != io.EOF {
				ls.hub.log.Debug("devops session output pump exit",
					"sessionId", ls.meta.SessionID, "stream", name, "error", err.Error())
			}
			return
		}
	}
}

// inputPump 输入泵：有界队列 → 终端 stdin（SSH 流控自然背压；写端永远
// 单 goroutine，io.Writer 并发纪律不破）。
func (ls *LiveSession) inputPump() {
	defer ls.pumpWG.Done()
	for chunk := range ls.inputCh {
		if _, err := ls.term.Stdin().Write(chunk); err != nil {
			// stdin 断 = 会话死：拆终端让 watch() 收束（不直接调 cleanup，
			// 防止 pumpWG.Wait 等本泵死锁）。
			ls.triggerCleanup("error", err)
			return
		}
	}
}

// fanOut 扇出到全部消费者（单写超时；超时即摘——慢消费者不拖死泵）。
func (ls *LiveSession) fanOut(p []byte) {
	ls.conMu.Lock()
	consumers := make([]*consumer, 0, len(ls.consumers))
	for _, c := range ls.consumers {
		consumers = append(consumers, c)
	}
	ls.conMu.Unlock()
	for _, c := range consumers {
		c.write(p, ls)
	}
}

// write 带超时的消费者写（超时/写错摘除该消费者；会话与存档不受影响）。
func (c *consumer) write(p []byte, ls *LiveSession) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.evicted {
		return
	}
	done := make(chan error, 1)
	go func() {
		_, err := c.w.Write(p)
		done <- err
	}()
	timer := time.NewTimer(c.timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != nil {
			ls.evict(c, "write error")
		}
	case <-timer.C:
		ls.evict(c, "write timeout")
	}
}

// evict 摘除消费者（幂等；审计留痕走 hub 日志，会话本身不受影响）。
func (ls *LiveSession) evict(c *consumer, why string) {
	ls.conMu.Lock()
	if _, exist := ls.consumers[c.id]; !exist {
		ls.conMu.Unlock()
		return
	}
	c.evicted = true
	delete(ls.consumers, c.id)
	ls.conMu.Unlock()
	ls.hub.log.Info("devops session consumer detached",
		"sessionId", ls.meta.SessionID, "reason", why)
}

// written 存档累计字节（Close 后仍可读）。
func (w *CastWriter) written() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bytes
}
