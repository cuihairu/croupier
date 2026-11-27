package tcp

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// Server implements a plain TCP request-response transport server.
type Server struct {
	config   *Config
	listener net.Listener
	// tcpListener 是 listener 的底层 TCP 监听：TLS 路径上 listener 是
	// crypto/tls 包装层（不实现 SetDeadline），accept deadline 必须设在
	// unwrap 结果上，否则 TLS 空闲时 Accept 永久阻塞、ctx 取消无人理会
	// （AUD-081）。可为 nil（测试直构 Server），Serve 内回退到明文断言。
	tcpListener *net.TCPListener
	handler     transportcore.Handler
	closing     chan struct{}
	once        sync.Once
	wg          sync.WaitGroup

	// mu/conns 跟踪活跃连接：Close 或 Serve ctx 取消（AUD-081）时主动断开，
	// 避免 wg.Wait / worker 被「阻塞在读直到 RecvTimeout」的 handler 卡住
	// （互联 RecvTimeout 90s 时优雅停机会拖满整个超时）。
	mu    sync.Mutex
	conns map[net.Conn]struct{}

	// negotiationWindow, when set (tests only), overrides the per-connection
	// accepted version window so N/N-1 mixes can be rehearsed before
	// Version1 moves (AUD-101). nil in production.
	negotiationWindow func() (uint8, uint8)
}

type connectionSessionContextKey struct{}

// ConnectionHandshakeKey is the per-connection session key used by the
// interconnect hello handshake to mark a connection as authenticated.
const ConnectionHandshakeKey = "interconnect-handshake"

// ConnectionSession carries per-connection state through Handler contexts.
// Transport ownership keeps the state bounded by the accepted connection's
// lifetime; no global registry is required.
type ConnectionSession struct {
	mu       sync.Mutex
	metadata map[string]any
}

// NewConnectionSession creates a fresh per-connection session for tests and
// callers that wire Handler contexts without an accepted net.Conn.
func NewConnectionSession() *ConnectionSession { return &ConnectionSession{} }

// WithConnectionSession attaches a session to a context (test/injection path).
func WithConnectionSession(ctx context.Context, session *ConnectionSession) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, connectionSessionContextKey{}, session)
}

// HandshakeDone reports whether the interconnect hello handshake completed.
func (s *ConnectionSession) HandshakeDone() bool {
	if s == nil {
		return false
	}
	done, _ := s.Get(ConnectionHandshakeKey)
	value, _ := done.(bool)
	return value
}

// Set stores a per-connection value.
func (s *ConnectionSession) Set(key string, value any) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.metadata == nil {
		s.metadata = make(map[string]any)
	}
	s.metadata[key] = value
}

// Get loads a per-connection value.
func (s *ConnectionSession) Get(key string) (any, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.metadata[key]
	return value, ok
}

// ConnectionSessionFromContext returns the session attached to a connection.
func ConnectionSessionFromContext(ctx context.Context) *ConnectionSession {
	if ctx == nil {
		return nil
	}
	session, _ := ctx.Value(connectionSessionContextKey{}).(*ConnectionSession)
	return session
}

var _ transportcore.Server = (*Server)(nil)

// NewServer creates a new TCP transport server.
func NewServer(config *Config, handler transportcore.Handler) (*Server, error) {
	if config == nil {
		config = &Config{}
	}
	if handler == nil {
		return nil, fmt.Errorf("handler is required")
	}

	ln, tcpLn, err := listen(config)
	if err != nil {
		return nil, err
	}

	return &Server{
		config:      config,
		listener:    ln,
		tcpListener: tcpLn,
		handler:     handler,
		closing:     make(chan struct{}),
	}, nil
}

// Serve accepts and handles connections until ctx is done or the server closes.
//
// 取消语义（AUD-081）：明文与 TLS 一致——accept deadline 设在底层
// *net.TCPListener 上，空闲 Accept 在 ≤1s 周期内解阻并检查 ctx；在途连接
// 由 ctx watcher 主动关闭，阻塞在 readFrame 上的 serveConn 立即返回并在
// 收尾 defer 中 join per-conn worker，停机有界、无 goroutine 泄漏。
func (s *Server) Serve(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// deadline 须设在底层 TCP listener 上：crypto/tls 包装层不透传
	// SetDeadline，只断言 s.listener 会漏掉 TLS 路径。
	deadlineListener := s.tcpListener
	if deadlineListener == nil {
		if tcpln, ok := s.listener.(*net.TCPListener); ok {
			deadlineListener = tcpln
		}
	}

	// ctx 取消 → 关闭全部在途连接（监听器退出仍走 Accept deadline 周期，
	// closing/listener 的生命周期归 Close）。watcher 随 Serve 返回退出，
	// 不给 Background ctx 留常驻 goroutine。
	serveDone := make(chan struct{})
	defer close(serveDone)
	go func() {
		select {
		case <-ctx.Done():
			s.closeConns()
		case <-serveDone:
		}
	}()

	for {
		if deadlineListener != nil {
			_ = deadlineListener.SetDeadline(time.Now().Add(time.Second))
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closing:
				return nil
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				continue
			}
			return fmt.Errorf("accept: %w", err)
		}

		s.trackConn(conn)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.untrackConn(conn)
			s.serveConn(ctx, conn)
		}()
	}
}

// inboundTask 是一条已解析、等待执行的请求。
type inboundTask struct {
	msgID uint32
	reqID uint32
	body  []byte
	ctx   context.Context
	// cancel 仅在配置了 RequestTimeout 时非空，响应写出后释放定时器。
	cancel context.CancelFunc
}

// connWriter 串行化单连接上的响应帧写（读循环 busy 内联回帧 + 执行
// worker 并发回帧共用一把连接级写锁）。响应帧携带连接协商版本（AUD-101）。
type connWriter struct {
	mu          sync.Mutex
	conn        net.Conn
	timeout     time.Duration
	negotiation *protocol.Negotiation
}

func (w *connWriter) writeResponse(msgID uint32, reqID uint32, body []byte) error {
	frame := protocol.NewMessageBodyWithVersion(w.negotiation.Version(), protocol.GetResponseMsgID(msgID), reqID, body)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timeout > 0 {
		_ = w.conn.SetWriteDeadline(time.Now().Add(w.timeout))
	}
	return writeFrame(w.conn, frame)
}

// writeResponseVersion stamps an explicit frame version (AUD-101): pre-lock
// rejection replies carry the peer's own offered version so an older peer
// can still read them.
func (w *connWriter) writeResponseVersion(msgID uint32, reqID uint32, body []byte, version uint8) error {
	frame := protocol.NewMessageBodyWithVersion(version, protocol.GetResponseMsgID(msgID), reqID, body)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timeout > 0 {
		_ = w.conn.SetWriteDeadline(time.Now().Add(w.timeout))
	}
	return writeFrame(w.conn, frame)
}

const (
	// defaultMaxInflightRequests 单连接业务执行管线上界（排队+执行中）。
	defaultMaxInflightRequests = 64
	// controlQueueLen 控制车道队列容量（控制 handler 廉价，永不 reject；
	// 与 sdks 客户端 ctrlInbox 同容量）。
	controlQueueLen = 64
)

// dispatchLanes 解出业务车道 worker 数与队列容量：queue+workers ≤
// maxInflight（max_inflight_requests 上界），且队列容量 ≥ 1——否则前几个
// 请求依赖 worker goroutine 的调度时序，低负载下可能出现伪 busy。
func (s *Server) dispatchLanes() (workers, queueLen int) {
	maxInflight := s.config.MaxInflightRequests
	if maxInflight <= 0 {
		maxInflight = defaultMaxInflightRequests
	}
	workers = s.config.HandlerWorkers
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers < 2 {
			workers = 2
		}
	}
	if workers > maxInflight-1 {
		workers = maxInflight - 1
	}
	if workers < 1 {
		workers = 1
	}
	return workers, maxInflight - workers
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	session := &ConnectionSession{}
	connCtx, cancelConn := context.WithCancel(context.WithValue(ctx, connectionSessionContextKey{}, session))

	// AUD-101：每条连接独立协商帧版本；测试可用 negotiationWindow 注入
	// 未来窗口（N/N-1 混跑演练），生产用包默认窗口。
	negotiation := protocol.NewNegotiation()
	if s.negotiationWindow != nil {
		minV, maxV := s.negotiationWindow()
		negotiation = protocol.NewNegotiationWindow(minV, maxV)
	}
	writer := &connWriter{conn: conn, timeout: s.config.SendTimeout, negotiation: negotiation}
	workers, queueLen := s.dispatchLanes()
	business := make(chan inboundTask, queueLen)
	control := make(chan inboundTask, controlQueueLen)
	var execWg sync.WaitGroup
	execWg.Add(workers + 1)
	for i := 0; i < workers; i++ {
		go func() {
			defer execWg.Done()
			s.executionWorker(writer, business)
		}()
	}
	go func() {
		defer execWg.Done()
		s.executionWorker(writer, control)
	}()

	// 收编顺序即退出安全性：先断连接（写阻塞的 worker 立即解阻）、
	// 再取消排队/执行中请求的 ctx（尊重 ctx 的 handler 快速返回）、
	// 最后关闭队列让 worker 排空缓冲后退出并 join。
	defer func() {
		_ = conn.Close()
		cancelConn()
		close(business)
		close(control)
		execWg.Wait()
	}()

	for {
		select {
		case <-connCtx.Done():
			return
		case <-s.closing:
			return
		default:
		}

		if s.config.RecvTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(s.config.RecvTimeout))
		}

		frame, err := readFrame(conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}

		version, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
		if err != nil {
			return
		}
		if !negotiation.Accepts(version) {
			// 仅拒绝请求帧（与 core mux/sdks 客户端同规则）：对响应帧回写会
			// 伪造对端从未发出的请求响应，对单向事件回写则可能形成响应循环。
			if protocol.IsRequest(msgID) {
				// 统一编码（AUD-002）：版本拒绝文档经标准编码器生成，
				// 沿用 InvokeResponse 载体（SDK 读取 payload 并报告范围）。
				// 拒绝帧以对端提供的版本回写（AUD-101），旧对端才读得到。
				rejection := tcptr.InvokeVersionRejectedBody(int(version), protocol.SupportedVersions())
				_ = writer.writeResponseVersion(msgID, reqID, rejection, version)
			}
			slog.Warn("rejected unsupported protocol version", "version", version, "msg_id", protocol.MsgIDString(msgID))
			continue
		}

		// AUD-101：首条受支持帧锁定连接协商版本；其后偏离已锁版本的帧按
		// 不支持同规则处理（粘滞），防止会话中途换版本破坏响应配对。
		if _, err := negotiation.Offer(version); err != nil {
			if protocol.IsRequest(msgID) {
				rejection := tcptr.InvokeVersionRejectedBody(int(version), protocol.SupportedVersions())
				_ = writer.writeResponseVersion(msgID, reqID, rejection, version)
			}
			slog.Warn("rejected conflicting frame version", "version", version,
				"negotiated", negotiation.Version(), "msg_id", protocol.MsgIDString(msgID))
			continue
		}

		// 执行解耦（CR-FIX-09）：读循环永不执行 handler——同步 Handle 会让
		// 慢 handler 卡死整条连接（心跳/取消等控制帧全部饿死）。双车道
		// （与 core mux/sdks 客户端同规则）：控制请求走专用车道永不
		// reject；业务队列满时立即回 busy 错误帧，读循环不阻塞、内存
		// 不积累，in-flight 恒 ≤ queue+workers。
		task := inboundTask{msgID: msgID, reqID: reqID, body: body}
		if s.config.RequestTimeout > 0 {
			task.ctx, task.cancel = context.WithTimeout(connCtx, s.config.RequestTimeout)
		} else {
			task.ctx = connCtx
		}
		if protocol.IsControlRequest(msgID) {
			select {
			case control <- task:
			case <-connCtx.Done():
				if task.cancel != nil {
					task.cancel()
				}
				return
			}
			continue
		}
		select {
		case business <- task:
		default:
			if task.cancel != nil {
				task.cancel()
			}
			slog.Warn("inbound queue full, answering busy", "msg_id", protocol.MsgIDString(msgID), "req_id", reqID, "max_inflight", queueLen+workers)
			_ = writer.writeResponse(msgID, reqID, busyResponseBody(msgID))
		}
	}
}

// executionWorker 消费指定车道：执行 handler 并回写响应帧（可乱序，
// 协议按 reqID 配对）。handler 错误仍回错误响应（原同步版语义，不吞
// 错误，真实错误进日志可诊断）。
func (s *Server) executionWorker(w *connWriter, queue <-chan inboundTask) {
	for task := range queue {
		respBody, handleErr := s.handler.Handle(task.ctx, task.msgID, task.reqID, task.body)
		if handleErr != nil {
			// Log the real error so it is diagnosable. The failure response is
			// encoded centrally (AUD-002): error text goes through the standard
			// JSON encoder and the response carries the failure channel of the
			// RPC's own wire type, so a client never reads a zero-value success.
			slog.Error("rpc handler error", "msg_id", protocol.MsgIDString(task.msgID), "error", handleErr)
			respBody = tcptr.RPCFailureBody(task.msgID, handleErr.Error())
		}
		_ = w.writeResponse(task.msgID, task.reqID, respBody)
		if task.cancel != nil {
			task.cancel()
		}
	}
}

// busyResponseBody 是业务队列打满时的即时拒绝响应体。经集中编码（AUD-002）：
// 有失败通道的 RPC 返回带 `Success:false` 的类型化响应，客户端可识别；无
// 失败通道的类型返回 nil（空 body，诚实边界见 core/transport/tcp/rpc_error.go）。
func busyResponseBody(msgID uint32) []byte {
	return tcptr.RPCFailureBody(msgID, "inbound queue full, retry on another instance")
}

// trackConn registers an accepted connection for Close-time shutdown.
func (s *Server) trackConn(conn net.Conn) {
	s.mu.Lock()
	if s.conns == nil {
		s.conns = map[net.Conn]struct{}{}
	}
	s.conns[conn] = struct{}{}
	s.mu.Unlock()
}

func (s *Server) untrackConn(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

// closeConns closes all tracked in-flight connections. Blocking readFrame
// calls fail immediately, and serveConn's deferred teardown closes both
// dispatch queues and joins its workers — this is what bounds shutdown for
// both ctx cancellation and Close (without it, an idle connection with no
// RecvTimeout pins serveConn + worker goroutines until process exit).
func (s *Server) closeConns() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns {
		_ = conn.Close()
	}
}

// Close stops accepting new connections and unblocks active handlers by
// closing their connections — wg.Wait otherwise waits out the full
// RecvTimeout on idle connections.
func (s *Server) Close() error {
	var closeErr error
	s.once.Do(func() {
		close(s.closing)
		closeErr = s.listener.Close()
		s.closeConns()
		s.wg.Wait()
	})
	return closeErr
}

// IsClosed reports whether the server has been closed.
func (s *Server) IsClosed() bool {
	select {
	case <-s.closing:
		return true
	default:
		return false
	}
}

// Addr returns the bound server address.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// listen 打开监听并返回（包装后 listener, 底层 *net.TCPListener, error）。
// TLS 路径用 tls.NewListener 包装自建的 TCP listener，第二个返回值是未包装
// 的底层监听——crypto/tls 包装层不透传 SetDeadline，Serve 的 accept
// deadline 只能设在它上面；明文路径两者是同一对象。
func listen(config *Config) (net.Listener, *net.TCPListener, error) {
	addr := normalizeAddr(config.Address)
	if addr == "" {
		addr = defaultAddress()
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	// net.Listen("tcp") 恒返回 *net.TCPListener；断言只为拿到具体类型，
	// 失败时 tcpLn 为 nil，Serve 回退到对 listener 的明文断言。
	tcpLn, _ := ln.(*net.TCPListener)

	if config.Insecure {
		return ln, tcpLn, nil
	}

	tlsConfig, err := tcptr.ServerTLSConfig(config)
	if err != nil {
		_ = ln.Close()
		return nil, nil, fmt.Errorf("create tls config: %w", err)
	}
	return tls.NewListener(ln, tlsConfig), tcpLn, nil
}

// createServerTLSConfig preserves the package-local entry point for the
// shared TLS listener configuration in core/transport/tcp.
func createServerTLSConfig(config *Config) (*tls.Config, error) { return tcptr.ServerTLSConfig(config) }

// BuildServerTLSConfig forwards to the shared TLS listener configuration.
func BuildServerTLSConfig(certFile, keyFile, caFile string, insecureSkipVerify bool) (*tls.Config, error) {
	return tcptr.BuildServerTLSConfig(certFile, keyFile, caFile, insecureSkipVerify)
}

// defaultAddress returns the listen address used when Config.Address is
// empty. Defaults to 127.0.0.1:19090; CROUPIER_TCP_ADDR overrides it so
// tests (and constrained environments) can pick a free port.
func defaultAddress() string {
	if addr := strings.TrimSpace(os.Getenv("CROUPIER_TCP_ADDR")); addr != "" {
		return addr
	}
	return "127.0.0.1:19090"
}
