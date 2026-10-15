package healthprobe

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

// 协议适配骨架（agent-health-recovery-review §5.5 层 2）：声明式多步交互运行时。
// 职责=连接生命周期、步骤推进、阶段标记、期望匹配、结果语义、传输层重试；
// 游戏协议知识一律不进此层（每游戏一份适配器=层 3，见 games/ 与 RegisterGameProtocol）。
//
// 内核是唯一失败计数权威：ProtocolChecker 只实现 Checker 契约，失败阈值/防抖/
// 窗口/上行全部复用 Probe，骨架不得自持计数器或状态机。
const (
	// DefaultStepTimeout 单步默认超时（§5.5.1：Timeout<=0 用默认 5s）。
	DefaultStepTimeout = 5 * time.Second
	// DefaultDialTimeout 建连默认超时（对齐 TCP 目标工厂默认 2s）。
	DefaultDialTimeout = 2 * time.Second
)

// StepExpect 步骤期望：字节级匹配，三形态覆盖静态私有协议握手（§5.5.1 契约）。
// 指定多个形态时全部须满足（AND）。加密、压缩、挑战-响应表达不了 → 升级层 3
// 代码适配，骨架不塞加密原语。
//
// 字节字段双形态：Contains/Prefix 给程序构造用；配置面（YAML/JSON）用
// containsHex/prefixHex 十六进制串（[]byte 的 yaml 缺省编码是 base64，对
// 写握手魔数的运维不友好），工厂归一到字节字段，双写报错。
type StepExpect struct {
	Contains    []byte `json:"contains,omitempty" yaml:"contains,omitempty"`
	Prefix      []byte `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	ContainsHex string `json:"containsHex,omitempty" yaml:"containsHex,omitempty"`
	PrefixHex   string `json:"prefixHex,omitempty" yaml:"prefixHex,omitempty"`
	Regexp      string `json:"regexp,omitempty" yaml:"regexp,omitempty"`
}

// StepSpec 单步：发什么、等什么、等多久。Name=阶段标记（进 Result.Details
// 与 LastReason）；Send 留空=纯等待（只收不发）；Expect 留空=任意响应即通过。
// Send/SendHex 双形态同 StepExpect（配置面用 sendHex）。
type StepSpec struct {
	Name    string        `json:"name,omitempty" yaml:"name,omitempty"`
	Send    []byte        `json:"send,omitempty" yaml:"send,omitempty"`
	SendHex string        `json:"sendHex,omitempty" yaml:"sendHex,omitempty"`
	Expect  StepExpect    `json:"expect,omitempty" yaml:"expect,omitempty"`
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"` // <=0 用 DefaultStepTimeout
}

// ProtocolChecker 骨架本体：实现 Checker——一次 Check=新建连接、走完步骤、
// 关连接（规则 1：不跨 Check 复用会话，无状态）。错误串携带阶段标记
// （protocol failed at step 2 (login): ...）落 Window.LastReason；阶段标记
// 同时以 Details（Detailing）出口：phase=login、step=2/5、reconnect=1。
type ProtocolChecker struct {
	Address     string // host:port（L2 v1 仅 TCP；HTTP 归 HTTPChecker）
	Steps       []StepSpec
	DialTimeout time.Duration // 建连超时；<=0 用 DefaultDialTimeout（配置侧取 target.timeout）

	// dialer 可选注入（测试用；生产 nil）。骨架只提供 dialer 注入——注入的
	// net.Conn 已可完全控制读写行为，无需另设 Reader 注入口。
	dialer func(ctx context.Context, network, addr string) (net.Conn, error)

	mu      sync.Mutex
	lastRun map[string]string // 最近一次 Check 的明细（Details 出口）
}

// NewProtocolChecker 构造声明式协议探测；步骤名、十六进制配置字段与正则在
// 构造期校验（配置 fail-fast，坏配置不留到探测期反复报）。
func NewProtocolChecker(address string, steps []StepSpec, dialTimeout time.Duration) (*ProtocolChecker, error) {
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("protocol checker: address required")
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("protocol checker: at least one step required")
	}
	normalized := make([]StepSpec, len(steps))
	for i, s := range steps {
		ns, err := normalizeStepSpec(s)
		if err != nil {
			return nil, fmt.Errorf("protocol checker: step %d: %w", i+1, err)
		}
		normalized[i] = ns
	}
	return &ProtocolChecker{Address: address, Steps: normalized, DialTimeout: dialTimeout}, nil
}

// normalizeStepSpec 归一单步：hex 配置字段解码进字节字段（双写报错），步骤名
// 与正则校验。构造期一次性完成，探测路径零解析开销。
func normalizeStepSpec(s StepSpec) (StepSpec, error) {
	if strings.TrimSpace(s.Name) == "" {
		return s, fmt.Errorf("name required (phase marker)")
	}
	if h := strings.TrimSpace(s.SendHex); h != "" {
		if len(s.Send) > 0 {
			return s, fmt.Errorf("(%s): send and sendHex are mutually exclusive", s.Name)
		}
		b, err := hex.DecodeString(h)
		if err != nil {
			return s, fmt.Errorf("(%s): invalid sendHex: %w", s.Name, err)
		}
		s.Send, s.SendHex = b, ""
	}
	e := &s.Expect
	if h := strings.TrimSpace(e.ContainsHex); h != "" {
		if len(e.Contains) > 0 {
			return s, fmt.Errorf("(%s): contains and containsHex are mutually exclusive", s.Name)
		}
		b, err := hex.DecodeString(h)
		if err != nil {
			return s, fmt.Errorf("(%s): invalid containsHex: %w", s.Name, err)
		}
		e.Contains, e.ContainsHex = b, ""
	}
	if h := strings.TrimSpace(e.PrefixHex); h != "" {
		if len(e.Prefix) > 0 {
			return s, fmt.Errorf("(%s): prefix and prefixHex are mutually exclusive", s.Name)
		}
		b, err := hex.DecodeString(h)
		if err != nil {
			return s, fmt.Errorf("(%s): invalid prefixHex: %w", s.Name, err)
		}
		e.Prefix, e.PrefixHex = b, ""
	}
	if e.Regexp != "" {
		if _, err := regexp.Compile(e.Regexp); err != nil {
			return s, fmt.Errorf("(%s): invalid regexp: %w", s.Name, err)
		}
	}
	return s, nil
}

// Check implements Checker：新建连接→逐步 Send/收→匹配→关连接。
func (c *ProtocolChecker) Check(ctx context.Context) error {
	details := map[string]string{"probeType": "protocol", "address": c.Address}
	defer func() {
		c.mu.Lock()
		c.lastRun = details
		c.mu.Unlock()
	}()

	// 规则 4：传输层重试仅限建连阶段瞬时错误（此刻未发出任何字节）单次重试。
	conn, retried, dialErr := c.dial(ctx)
	if retried {
		details["reconnect"] = "1"
	}
	if conn == nil {
		details["phase"] = "dial"
		return fmt.Errorf("dial tcp %s: %w", c.Address, dialErr)
	}
	defer func() { _ = conn.Close() }()

	for i, step := range c.Steps {
		stepTimeout := step.Timeout
		if stepTimeout <= 0 {
			stepTimeout = DefaultStepTimeout
		}
		if err := c.runStep(ctx, conn, step, stepTimeout); err != nil {
			// 规则 2/3：阶段标记进错误串（→LastReason）与 Details。握手半途
			// 不重试（非幂等：登录半状态），失败即整次 Check 失败。
			details["phase"] = step.Name
			details["step"] = fmt.Sprintf("%d/%d", i+1, len(c.Steps))
			return fmt.Errorf("protocol failed at step %d (%s): %w", i+1, step.Name, err)
		}
	}
	details["phase"] = "done"
	details["step"] = fmt.Sprintf("%d/%d", len(c.Steps), len(c.Steps))
	return nil
}

// Details implements Detailing：最近一次 Check 的阶段标记明细（ProbeOnce 探测
// 到即写 Result.Details，随后随失败明细上行）。从未执行过 Check 时只含探测
// 配置面（无 phase）。
func (c *ProtocolChecker) Details() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]string{"probeType": "protocol", "address": c.Address}
	for k, v := range c.lastRun {
		out[k] = v
	}
	return out
}

// dial 建连：瞬时失败单次重试（规则 4）。retried=true 表示重试过（成功与否），
// 调用方记 reconnect=1；err 为最终失败原因（重试过则取重试那次）。
func (c *ProtocolChecker) dial(ctx context.Context) (net.Conn, bool, error) {
	dial := c.dialer
	if dial == nil {
		d := &net.Dialer{Timeout: c.dialTimeout()}
		dial = d.DialContext
	}
	conn, err := dial(ctx, "tcp", c.Address)
	if err == nil {
		return conn, false, nil
	}
	if ctx.Err() != nil {
		return nil, false, err // 上层取消不算瞬时错误，重试只会放大取消延迟
	}
	conn, err = dial(ctx, "tcp", c.Address)
	if err != nil {
		return nil, true, err
	}
	return conn, true, nil
}

func (c *ProtocolChecker) dialTimeout() time.Duration {
	if c.DialTimeout > 0 {
		return c.DialTimeout
	}
	return DefaultDialTimeout
}

// runStep 执行单步：Send（若有）→ 流式收包直到匹配或超时/EOF。
// 期望匹配只做字节级（规则 5）：Contains/Prefix/Regexp 全缓冲判定，指定多个
// 形态时 AND。期望留空=收到 ≥1 字节即通过。每步读缓冲独立（上一步残余字节
// 不跨步携带——一步多包的协议属层 3 适配范畴）。
func (c *ProtocolChecker) runStep(ctx context.Context, conn net.Conn, step StepSpec, stepTimeout time.Duration) error {
	deadline := time.Now().Add(stepTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d // 上层 ctx 更早到期时以更紧者为准
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}
	if len(step.Send) > 0 {
		if _, err := conn.Write(step.Send); err != nil {
			return fmt.Errorf("write %d bytes: %w", len(step.Send), err)
		}
	}
	matcher, err := newStepMatcher(step.Expect)
	if err != nil {
		return err
	}
	buf := make([]byte, 0, 512)
	tmp := make([]byte, 4096)
	for {
		n, rerr := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if matcher(buf) {
				return nil
			}
		}
		if rerr != nil {
			// 超时/EOF 都是终态：无法再收到更多字节，按收到的现场报。
			if isTimeoutErr(rerr) {
				if len(buf) == 0 {
					return fmt.Errorf("timeout waiting for response")
				}
				return fmt.Errorf("timeout waiting for match (got %d bytes: %q)", len(buf), clipBytes(buf))
			}
			if errors.Is(rerr, io.EOF) {
				if len(buf) == 0 {
					return fmt.Errorf("connection closed without response")
				}
				return fmt.Errorf("connection closed while waiting (got %d bytes: %q)", len(buf), clipBytes(buf))
			}
			return fmt.Errorf("read: %w", rerr)
		}
	}
}

// stepMatcher 返回全缓冲判定函数；Regexp 非法在探测期报（构造期已拦截配置路径）。
func newStepMatcher(e StepExpect) (func([]byte) bool, error) {
	var re *regexp.Regexp
	if e.Regexp != "" {
		var err error
		re, err = regexp.Compile(e.Regexp)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %w", err)
		}
	}
	return func(buf []byte) bool {
		if len(e.Contains) > 0 && !bytes.Contains(buf, e.Contains) {
			return false
		}
		if len(e.Prefix) > 0 && !bytes.HasPrefix(buf, e.Prefix) {
			return false
		}
		if re != nil && !re.Match(buf) {
			return false
		}
		// 全形态为空=任意响应即通过（buf 非空由调用方保证）。
		return true
	}, nil
}

// isTimeoutErr 判定读超时：net.Error 且 Timeout()=true（os.ErrDeadlineExceeded
// 经 deadline 触发的读失败也实现该接口，统一走接口判定而非错误串匹配）。
func isTimeoutErr(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// clipBytes 诊断串里最多带 64 字节现场，防止大响应刷爆 LastReason。
func clipBytes(buf []byte) []byte {
	if len(buf) > 64 {
		return buf[:64]
	}
	return buf
}

// GameProtocolMaker 游戏协议适配器构造器（层 3 接入点）：从目标声明构造
// Checker。适配器包在 init() 里注册，装配侧 blank import（db driver 先例）。
type GameProtocolMaker func(t TargetConfig) (Checker, error)

var (
	gameProtocolMu     sync.RWMutex
	gameProtocolMakers = map[string]GameProtocolMaker{}
)

// RegisterGameProtocol 注册游戏协议适配器（层 3 接入点）。重复注册=装配错误，
// 直接 panic（database/sql driver 注册同口径）。
func RegisterGameProtocol(name string, maker GameProtocolMaker) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		panic("healthprobe: RegisterGameProtocol name is required")
	}
	if maker == nil {
		panic("healthprobe: RegisterGameProtocol maker is nil")
	}
	gameProtocolMu.Lock()
	defer gameProtocolMu.Unlock()
	if _, dup := gameProtocolMakers[name]; dup {
		panic("healthprobe: RegisterGameProtocol called twice for " + name)
	}
	gameProtocolMakers[name] = maker
}

// RegisteredGameProtocols 返回已注册的游戏协议名（诊断用）。
func RegisteredGameProtocols() []string {
	gameProtocolMu.RLock()
	defer gameProtocolMu.RUnlock()
	out := make([]string, 0, len(gameProtocolMakers))
	for name := range gameProtocolMakers {
		out = append(out, name)
	}
	return out
}

func lookupGameProtocol(name string) (GameProtocolMaker, bool) {
	gameProtocolMu.RLock()
	defer gameProtocolMu.RUnlock()
	maker, ok := gameProtocolMakers[strings.ToLower(strings.TrimSpace(name))]
	return maker, ok
}
