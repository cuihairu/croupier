// Package minecraft 层 3 游戏协议适配器：Minecraft Java 版 Server List Ping（SLP）。
//
// 实现按公开协议规范（wiki.vg「Server List Ping」）自写，零第三方代码/数据
// 文件（game-data-sources.md 红线：PrismarineJS/minecraft-data 只作协议事实
// 参考，不入库不分发）；仅用标准库，core 零外部依赖约束保持。
//
// SLP 是只读状态查询：握手(state=1)→Status Request→Status Response(JSON)，
// 无需登录/加密（online-mode 的加密链在 Login 流程，SLP 不涉及——本适配器
// 因此可零账号验证任意 Java 版服务器；「待真机账号」卡点只挡 Login 级探测）。
package minecraft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	healthprobe "github.com/cuihairu/croupier/core/healthprobe"
)

const (
	// DefaultSLPTimeout 整轮握手+状态交换超时（两包交互，5s 与内核单步默认同档）。
	DefaultSLPTimeout = 5 * time.Second
	// DefaultProtocolVersion 握手声明的客户端协议版本（47=1.8.9）。服务器对
	// status 请求不校验版本（版本协商发生在 Login 流程），vanilla/Paper 对
	// 任意声明值均正常回 Status Response；个别代理实现可能严格——已知边界。
	DefaultProtocolVersion = 47
	// DefaultPort 地址缺端口时的 Minecraft Java 默认端口。
	DefaultPort = 25565
	// maxStatusBytes 状态响应长度上限（favicon 场景实测 KB 级，2 MiB 为宽上限，
	// 防异常服务器把探测拖成内存事故）。
	maxStatusBytes = 2 << 20
)

// SLP status 流的包 ID（handshake/request/response 三个阶段包 ID 均为 0x00，
// 各自独立常量以承载协议语义）。
const (
	slpPacketHandshake = 0x00 // 握手包（nextState=1 进 status）
	slpPacketRequest   = 0x00 // Status Request（无载荷）
	slpPacketResponse  = 0x00 // Status Response
)

// SLPChecker Minecraft Java SLP 探测：实现 healthprobe.Checker + Detailing。
// 一次 Check=新建连接、SLP 全流程、关连接（与骨架同规则：不跨 Check 复用会话）。
type SLPChecker struct {
	Address         string        // host:port；缺端口补 DefaultPort
	Timeout         time.Duration // 整轮交互超时；<=0 用 DefaultSLPTimeout
	ProtocolVersion int           // 握手声明版本；<=0 用 DefaultProtocolVersion

	// dialer 可选注入（测试用；生产 nil）。
	dialer func(ctx context.Context, network, addr string) (net.Conn, error)

	mu      sync.Mutex
	lastRun map[string]string
}

// NewSLPChecker 构造 Minecraft SLP 探测。
func NewSLPChecker(address string, timeout time.Duration) *SLPChecker {
	return &SLPChecker{Address: address, Timeout: timeout}
}

func init() {
	healthprobe.RegisterGameProtocol("minecraft", func(t healthprobe.TargetConfig) (healthprobe.Checker, error) {
		return NewSLPChecker(t.Address, t.Timeout), nil
	})
}

// Check implements healthprobe.Checker。
func (c *SLPChecker) Check(ctx context.Context) error {
	details := map[string]string{"probeType": "minecraft", "address": c.Address}
	defer func() {
		c.mu.Lock()
		c.lastRun = details
		c.mu.Unlock()
	}()

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultSLPTimeout
	}
	conn, err := c.dialSLP(ctx, timeout)
	if err != nil {
		details["phase"] = "dial"
		return fmt.Errorf("minecraft slp: dial %s: %w", c.Address, err)
	}
	defer func() { _ = conn.Close() }()
	// 整轮交互共用一个绝对期限（两包交换，无需按包分段计时）。
	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		details["phase"] = "handshake"
		return fmt.Errorf("minecraft slp: handshake: set deadline: %w", err)
	}

	host, port, err := splitSLPAddress(c.Address)
	if err != nil {
		details["phase"] = "handshake"
		return fmt.Errorf("minecraft slp: handshake: %w", err)
	}
	hs := buildHandshakePacket(c.protocolVersion(), host, port)
	req := buildStatusRequestPacket()
	if _, err := conn.Write(append(hs, req...)); err != nil {
		details["phase"] = "handshake"
		return fmt.Errorf("minecraft slp: handshake: write: %w", err)
	}

	statusJSON, err := readStatusResponse(conn)
	if err != nil {
		details["phase"] = "status"
		return fmt.Errorf("minecraft slp: status: %w", err)
	}
	ver, players, err := parseStatus(statusJSON)
	if err != nil {
		details["phase"] = "status"
		return fmt.Errorf("minecraft slp: status: %w", err)
	}
	details["phase"] = "done"
	if ver.Name != "" {
		details["version"] = ver.Name
	}
	if ver.Protocol != 0 {
		details["serverProtocol"] = strconv.Itoa(ver.Protocol)
	}
	if players.Online > 0 || players.Max > 0 {
		details["players"] = fmt.Sprintf("%d/%d", players.Online, players.Max)
	}
	return nil
}

// Details implements healthprobe.Detailing：最近一次 Check 的阶段明细。
func (c *SLPChecker) Details() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]string{"probeType": "minecraft", "address": c.Address}
	for k, v := range c.lastRun {
		out[k] = v
	}
	return out
}

// slpStatus 版本/玩家摘要（只取探测需要的字段，不做完整协议建模）。
type slpStatus struct {
	Name     string
	Protocol int
}
type slpPlayers struct {
	Online int
	Max    int
}

func (c *SLPChecker) protocolVersion() int {
	if c.ProtocolVersion > 0 {
		return c.ProtocolVersion
	}
	return DefaultProtocolVersion
}

func (c *SLPChecker) dialSLP(ctx context.Context, timeout time.Duration) (net.Conn, error) {
	addr, err := resolveSLPAddress(c.Address)
	if err != nil {
		return nil, err
	}
	dial := c.dialer
	if dial == nil {
		d := &net.Dialer{Timeout: timeout}
		dial = d.DialContext
	}
	return dial(ctx, "tcp", addr)
}

// resolveSLPAddress 补默认端口；裸 IPv6 由 JoinHostPort 自动补方括号。
func resolveSLPAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", errors.New("address required")
	}
	if _, _, err := net.SplitHostPort(address); err == nil {
		return address, nil
	} else if isSplitAddrErr(err) {
		// 缺端口（host）或裸 IPv6（::1）都补默认端口成合法 host:port。
		return net.JoinHostPort(address, strconv.Itoa(DefaultPort)), nil
	} else {
		return "", fmt.Errorf("invalid address %q: %w", address, err)
	}
}

func isSplitAddrErr(err error) bool {
	var ae *net.AddrError
	return errors.As(err, &ae) &&
		(ae.Err == "missing port in address" || ae.Err == "too many colons in address")
}

// splitSLPAddress 拆出握手包所需的 host（不带端口）与 port。
func splitSLPAddress(address string) (string, int, error) {
	resolved, err := resolveSLPAddress(address)
	if err != nil {
		return "", 0, err
	}
	host, portStr, err := net.SplitHostPort(resolved)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port %q", portStr)
	}
	return host, port, nil
}

// appendVarInt 追加 LEB128 变长整数（Minecraft 协议 VarInt，≤5 字节）。
func appendVarInt(b []byte, v uint32) []byte {
	for {
		if v&^0x7F == 0 {
			return append(b, byte(v))
		}
		b = append(b, byte(v&0x7F)|0x80)
		v >>= 7
	}
}

// readVarInt 读 VarInt（≤5 字节，超长即坏包）。
func readVarInt(r *bytes.Reader) (uint32, error) {
	var result uint32
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("varint: %w", err)
		}
		result |= uint32(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			return result, nil
		}
	}
	return 0, errors.New("varint too long")
}

// slpFrame 包帧：VarInt(载荷长度) + 载荷；载荷=VarInt(包 ID) + 数据。
func slpFrame(packetID byte, payload []byte) []byte {
	data := append([]byte{packetID}, payload...)
	frame := appendVarInt(nil, uint32(len(data)))
	return append(frame, data...)
}

// buildHandshakePacket 构造 Handshake（state=1 status）：VarInt 协议版本、
// String 服务器地址（host 不带端口）、u16 大端端口、VarInt nextState=1。
func buildHandshakePacket(protocolVersion int, host string, port int) []byte {
	var payload []byte
	payload = appendVarInt(payload, uint32(protocolVersion))
	payload = appendVarInt(payload, uint32(len(host)))
	payload = append(payload, host...)
	payload = append(payload, byte(port>>8), byte(port))
	payload = appendVarInt(payload, 1) // nextState: 1=status
	return slpFrame(slpPacketHandshake, payload)
}

// buildStatusRequestPacket 构造 Status Request（status 流 0x00，无载荷）。
func buildStatusRequestPacket() []byte {
	return slpFrame(slpPacketRequest, nil)
}

// readStatusResponse 读 Status Response 帧并剥出 JSON 串（VarInt 长度前缀
// + VarInt 包 ID + VarInt 串长 + UTF-8 JSON）。
func readStatusResponse(conn net.Conn) ([]byte, error) {
	length, err := readConnVarInt(conn)
	if err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}
	if length == 0 || length > maxStatusBytes {
		return nil, fmt.Errorf("invalid frame length %d", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}
	r := bytes.NewReader(payload)
	id, err := readVarInt(r)
	if err != nil {
		return nil, fmt.Errorf("read packet id: %w", err)
	}
	if id != slpPacketResponse {
		return nil, fmt.Errorf("unexpected packet id 0x%x", id)
	}
	strLen, err := readVarInt(r)
	if err != nil {
		return nil, fmt.Errorf("read string length: %w", err)
	}
	if uint64(strLen) > uint64(r.Len()) {
		return nil, fmt.Errorf("string length %d exceeds payload %d", strLen, r.Len())
	}
	statusJSON := make([]byte, strLen)
	if _, err := io.ReadFull(r, statusJSON); err != nil {
		return nil, fmt.Errorf("read status json: %w", err)
	}
	return statusJSON, nil
}

func readConnVarInt(conn net.Conn) (uint32, error) {
	var result uint32
	one := make([]byte, 1)
	for i := 0; i < 5; i++ {
		if _, err := io.ReadFull(conn, one); err != nil {
			return 0, fmt.Errorf("varint: %w", err)
		}
		result |= uint32(one[0]&0x7F) << (7 * i)
		if one[0]&0x80 == 0 {
			return result, nil
		}
	}
	return 0, errors.New("varint too long")
}

// parseStatus 解析 Status JSON：version 键必须存在（SLP 已验证的硬标准）；
// players 可缺省（部分代理省略）。
func parseStatus(statusJSON []byte) (slpStatus, slpPlayers, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(statusJSON, &raw); err != nil {
		return slpStatus{}, slpPlayers{}, fmt.Errorf("invalid json: %w", err)
	}
	verRaw, ok := raw["version"]
	if !ok {
		return slpStatus{}, slpPlayers{}, errors.New("response missing version")
	}
	var status slpStatus
	if err := json.Unmarshal(verRaw, &status); err != nil {
		return slpStatus{}, slpPlayers{}, fmt.Errorf("malformed version: %w", err)
	}
	var players slpPlayers
	if pRaw, ok := raw["players"]; ok {
		if err := json.Unmarshal(pRaw, &players); err != nil {
			// players 解析失败不判失败：SLP 验证以 version 为准。
			players = slpPlayers{}
		}
	}
	return status, players, nil
}
