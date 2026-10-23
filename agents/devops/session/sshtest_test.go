package session

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

// 在进程内起一台真实 SSH 服务端（x/crypto/ssh server side，非 mock——走完整
// SSH 协议：握手/认证/频道/pty/窗口调整）。echo 行为：stdin → stdout 原样
// 回显（telnet 式），供端到端数据流断言。

// echoSSHServer 测试用 SSH 服务端。
type echoSSHServer struct {
	t           *testing.T
	cfg         *ssh.ServerConfig
	ln          net.Listener
	mu          sync.Mutex
	resizes     [][2]int
	ptyRequests int
	closeCh     chan struct{}
	wg          sync.WaitGroup
}

// newEchoSSHServer 起本机随机端口 SSH 服务端（口令认证 user/pass）。
func newEchoSSHServer(t *testing.T, user, pass string) *echoSSHServer {
	t.Helper()
	signer, err := ssh.ParsePrivateKey(testHostKeyPEM)
	if err != nil {
		t.Fatalf("parse host key: %v", err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == user && string(password) == pass {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("auth rejected for %q", conn.User())
		},
	}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &echoSSHServer{t: t, cfg: cfg, ln: ln, closeCh: make(chan struct{})}
	s.wg.Add(1)
	go s.serve()
	t.Cleanup(s.Close)
	return s
}

// Addr 服务端地址（host:port）。
func (s *echoSSHServer) Addr() string { return s.ln.Addr().String() }

// HostPort 拆出 host 与 port。
func (s *echoSSHServer) HostPort() (string, int) {
	addr := s.ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

// port 服务端口。
func (s *echoSSHServer) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

// Resizes 记录到的窗口调整序列。
func (s *echoSSHServer) Resizes() [][2]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][2]int(nil), s.resizes...)
}

// PTYRequests 收到的 pty 请求次数。
func (s *echoSSHServer) PTYRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ptyRequests
}

func (s *echoSSHServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(conn)
		}()
	}
}

func (s *echoSSHServer) handle(conn net.Conn) {
	sconn, chans, reqs, err := ssh.NewServerConn(conn, s.cfg)
	if err != nil {
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			return
		}
		s.handleChannel(ch, chReqs)
	}
}

func (s *echoSSHServer) handleChannel(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	hasPTY := false
	for req := range reqs {
		switch req.Type {
		case "pty-req":
			hasPTY = true
			s.mu.Lock()
			s.ptyRequests++
			s.mu.Unlock()
			_ = req.Reply(hasPTY, nil)
		case "shell":
			if !hasPTY {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)
			// echo：stdin → stdout，EOF 结束（goroutine——请求循环必须继续
			// 服务 window-change 等后续请求）。
			go func() {
				buf := make([]byte, 1024)
				for {
					n, rerr := ch.Read(buf)
					if n > 0 {
						if _, werr := ch.Write(buf[:n]); werr != nil {
							return
						}
					}
					if rerr != nil {
						return
					}
				}
			}()
		case "window-change":
			var wh struct {
				Cols uint32
				Rows uint32
				Wpx  uint32
				Hpx  uint32
			}
			if err := ssh.Unmarshal(req.Payload, &wh); err == nil {
				s.mu.Lock()
				s.resizes = append(s.resizes, [2]int{int(wh.Cols), int(wh.Rows)})
				s.mu.Unlock()
			}
			_ = req.Reply(true, nil)
		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
}

func (s *echoSSHServer) Close() {
	_ = s.ln.Close()
	close(s.closeCh)
	s.wg.Wait()
}

// testHostKeyPEM / testClientKeyPEM 每次测试进程运行时生成的 ed25519 密钥
// （host key 与 client key 独立；指纹断言不依赖固定值）。
var (
	testHostKeyPEM   []byte
	testClientKeyPEM []byte
)

func init() {
	testHostKeyPEM = mustGenKeyPEM()
	testClientKeyPEM = mustGenKeyPEM()
}

func mustGenKeyPEM() []byte {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "croupier-devops-test")
	if err != nil {
		panic(err)
	}
	return pem.EncodeToMemory(block)
}
