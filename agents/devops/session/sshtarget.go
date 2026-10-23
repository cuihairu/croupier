package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
)

// ioPipe 双端 Close 语义的 io.Pipe 别名（Close 读端 = 写端 ErrClosedPipe，
// Close 写端 = 读端 EOF——断线清理拆链靠它收束所有泵）。
func ioPipe() (r io.ReadCloser, w io.WriteCloser) {
	pr, pw := io.Pipe()
	return pr, pw
}

// onceErr 幂等且带返回值的 sync.Once（Wait/Close 竞争收敛用）。
type onceErr struct {
	once sync.Once
	err  error
}

func (o *onceErr) Do(f func() error) error {
	o.once.Do(func() { o.err = f() })
	return o.err
}

// SSHConfig SSH 通道凭据与主机指纹策略（装配自 config.Devops.SSH）。
type SSHConfig struct {
	// User SSH 登录用户。
	User string
	// PrivateKeyFile 私钥 PEM 路径（与 PasswordFile 至少一）。
	PrivateKeyFile string
	// PasswordFile 口令文件路径（首行即口令；文件化避免口令进 YAML/日志）。
	PasswordFile string
	// KnownHostsFile OpenSSH known_hosts 信任锚（与 InsecureHostKey 二选一）。
	KnownHostsFile string
	// InsecureHostKey 跳过 host key 校验（测试逃生门，启动时显式告警）。
	InsecureHostKey bool
	// ConnectTimeout TCP 连接+握手超时（0 = 10s）。
	ConnectTimeout time.Duration
}

// sshChannelName 通道名（审计与存档 channel 字段取值）。
const sshChannelName = "ssh"

// sshTerm pty 终端类型。
const sshTerm = "xterm-2560"

// sshTarget SSH 协议通道（业务插件位 Target 的真实实现，替代骨架 noop）：
// 白名单/审计由 Broker 统一前置，本实现只管 SSH 协议本身——TCP+握手（有界
// 超时）→ host key 校验 → 公钥/口令认证 → pty+shell。
type sshTarget struct {
	cfg      SSHConfig
	signer   ssh.Signer
	password string // 空串 = 不启用口令认证
	hkCB     ssh.HostKeyCallback
}

// NewSSHChannelTarget 装配 SSH 通道。构造期即装载凭据与指纹策略（fail-fast：
// 私钥读不了/口令文件空/指纹策略缺一，装配失败就不起会话面，不拖到第一次
// Open 才爆）。
func NewSSHChannelTarget(cfg SSHConfig) (Target, error) {
	if strings.TrimSpace(cfg.User) == "" {
		return nil, errors.New("session: ssh user is required")
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	t := &sshTarget{cfg: cfg}

	if cfg.PrivateKeyFile != "" {
		pem, err := os.ReadFile(cfg.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("session: read ssh private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(pem)
		if err != nil {
			return nil, fmt.Errorf("session: parse ssh private key: %w", err)
		}
		t.signer = signer
	}
	if cfg.PasswordFile != "" {
		raw, err := os.ReadFile(cfg.PasswordFile)
		if err != nil {
			return nil, fmt.Errorf("session: read ssh password file: %w", err)
		}
		// 首行即口令；空文件按缺凭据拒（写错路径不至于退化成无口令尝试）。
		line := strings.SplitN(string(raw), "\n", 2)[0]
		t.password = strings.TrimRight(line, "\r")
		if t.password == "" {
			return nil, errors.New("session: ssh password file is empty")
		}
	}
	if t.signer == nil && t.password == "" {
		return nil, errors.New("session: ssh requires private key or password")
	}

	switch {
	case cfg.InsecureHostKey:
		// 测试逃生门：显式选择才进这里；生产配置校验层要求 knownHostsFile。
		t.hkCB = ssh.InsecureIgnoreHostKey()
	case cfg.KnownHostsFile != "":
		cb, err := knownhosts.New(cfg.KnownHostsFile)
		if err != nil {
			return nil, fmt.Errorf("session: load known_hosts: %w", err)
		}
		t.hkCB = ssh.HostKeyCallback(cb)
	default:
		return nil, errors.New("session: ssh host key policy required (knownHostsFile or insecureHostKey)")
	}
	return t, nil
}

// Name 通道名。
func (t *sshTarget) Name() string { return sshChannelName }

// Open 建立一条真实 SSH pty 会话。
func (t *sshTarget) Open(ctx context.Context, req SessionRequest) (Session, error) {
	if req.Protocol != ProtocolSSH {
		return nil, fmt.Errorf("session: channel %s does not support protocol %s", sshChannelName, req.Protocol)
	}
	addr := net.JoinHostPort(req.Host, fmt.Sprint(req.Port))
	dialer := &net.Dialer{Timeout: t.cfg.ConnectTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("session: dial %s: %w", addr, err)
	}
	// 握手期也有界：挂在 ctx 与 ConnectTimeout 较早者上（对端吞握手不挂死）。
	deadline := time.Now().Add(t.cfg.ConnectTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	cfg := &ssh.ClientConfig{
		User:            t.cfg.User,
		Auth:            t.authMethods(),
		HostKeyCallback: t.hkCB,
		Timeout:         t.cfg.ConnectTimeout,
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	_ = conn.SetDeadline(time.Time{})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("session: ssh handshake %s: %w", addr, err)
	}
	client := ssh.NewClient(clientConn, chans, reqs)

	sess, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("session: open ssh session: %w", err)
	}

	cols, rows := req.Cols, req.Rows
	if cols <= 0 {
		cols = DefaultCols
	}
	if rows <= 0 {
		rows = DefaultRows
	}
	if err := sess.RequestPty(sshTerm, rows, cols, ssh.TerminalModes{}); err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, fmt.Errorf("session: request pty: %w", err)
	}
	// 管道化三向数据流：读写各持独立 pipe，Hub 泵单点消费/生产。
	stdinR, stdinW := ioPipe()
	stdoutR, stdoutW := ioPipe()
	stderrR, stderrW := ioPipe()
	sess.Stdin = stdinR
	sess.Stdout = stdoutW
	sess.Stderr = stderrW
	if err := sess.Shell(); err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, fmt.Errorf("session: start shell: %w", err)
	}

	return &sshTerminal{
		id:      req.SessionID,
		client:  client,
		sess:    sess,
		stdinW:  stdinW,
		stdoutR: stdoutR,
		stderrR: stderrR,
		stdoutW: stdoutW,
		stderrW: stderrW,
		stdinR:  stdinR,
		cols:    cols,
		rows:    rows,
	}, nil
}

// authMethods 装配认证方法（密钥在前、口令兜底；都不配构造期已拒）。
func (t *sshTarget) authMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod
	if t.signer != nil {
		methods = append(methods, ssh.PublicKeys(t.signer))
	}
	if t.password != "" {
		methods = append(methods, ssh.Password(t.password))
	}
	return methods
}

// sshTerminal Session+Terminal 的 SSH 实现。
type sshTerminal struct {
	id     string
	client *ssh.Client
	sess   *ssh.Session

	stdinW  io.WriteCloser
	stdoutR io.ReadCloser
	stderrR io.ReadCloser
	// 保留写端引用：Close 时整体拆链，读端以 EOF 收尾。
	stdoutW io.WriteCloser
	stderrW io.WriteCloser
	stdinR  io.ReadCloser

	cols, rows int

	closeOnce onceErr
	waitOnce  onceErr
}

func (t *sshTerminal) ID() string { return t.id }

// Stdin 终端输入写入端。
func (t *sshTerminal) Stdin() io.Writer { return t.stdinW }

// Stdout 终端输出读出端。
func (t *sshTerminal) Stdout() io.Reader { return t.stdoutR }

// Stderr 终端错误输出读出端。
func (t *sshTerminal) Stderr() io.Reader { return t.stderrR }

// Resize 调整远端 pty 窗口。
func (t *sshTerminal) Resize(cols, rows int) error {
	if err := ValidateWindow(cols, rows); err != nil {
		return err
	}
	if err := t.sess.WindowChange(rows, cols); err != nil {
		return fmt.Errorf("session: window change: %w", err)
	}
	t.cols, t.rows = cols, rows
	return nil
}

// Wait 等待远端会话结束（EOF 后拆 pipe 读端，泵以 EOF 收尾）。
func (t *sshTerminal) Wait() (TerminalExit, error) {
	exit := TerminalExit{ExitCode: -1}
	err := t.waitOnce.Do(func() error {
		werr := t.sess.Wait()
		// 会话结束即拆链：Close 与 Wait 竞争时保证幂等。
		t.closeAll()
		if werr == nil {
			exit.ExitCode = 0
			return nil
		}
		var ee *ssh.ExitError
		if errors.As(werr, &ee) {
			exit.ExitCode = ee.ExitStatus()
			return nil
		}
		return werr
	})
	return exit, err
}

// Close 本地断开（幂等）：会话+连接+管道全拆。
func (t *sshTerminal) Close() error {
	return t.closeOnce.Do(func() error {
		_ = t.sess.Close()
		_ = t.client.Close()
		t.closeAll()
		return nil
	})
}

func (t *sshTerminal) closeAll() {
	_ = t.stdinW.Close()
	_ = t.stdoutW.Close()
	_ = t.stderrW.Close()
	_ = t.stdinR.Close()
	_ = t.stdoutR.Close()
	_ = t.stderrR.Close()
}
