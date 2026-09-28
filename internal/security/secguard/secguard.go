// Package secguard 出站安全守卫（OPEN-ISSUES #56）。
//
// 四个 L3 键（sec.*）在此收敛为可执行语义：
//   - sec.allowPorts（string，逗号分隔端口白名单；空 = 不限）
//   - sec.allowIPs（string，逗号分隔 IP/CIDR 放行清单，配合 SSRF 保护）
//   - sec.domainFilter（string，逗号分隔域名后缀白名单；空 = 不限）
//   - sec.ssrfProtection（bool；true = 外呼拒绝私有/回环/链路本地地址，
//     除非命中 sec.allowIPs）
//
// 边界（诚实）：守卫只作用于用户可配置 URL 的出站 HTTP——通知 webhook
// （钉钉/飞书/企微/通用）与检查更新拉取；agent/DB/SDK 通道不经过本包。
// 默认全关（零行为变更），开启后由 CheckURL 静态校验 + HTTPClient 拨号
// Control 钩子双层拦截（后者消除 DNS 解析 TOCTOU）。
package secguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/internal/platform/settings"
)

// Settings 四键的解析结果（L3∧L2∧默认）。
type Settings struct {
	AllowPorts     []int
	AllowIPs       []string
	AllowDomains   []string
	SSRFProtection bool
}

// Resolve 从分层设置解析守卫参数（l 为 nil 时返回全关零值——settings 未
// 初始化的测试/早期启动路径直通）。
func Resolve(l *settings.Layered) Settings {
	if l == nil {
		return Settings{}
	}
	str := func(key string) string {
		v, _, _ := l.GetString(context.Background(), key)
		return v
	}
	return Settings{
		AllowPorts:     parsePortList(str(settings.KeySecAllowPorts)),
		AllowIPs:       parseList(str(settings.KeySecAllowIPs)),
		AllowDomains:   parseList(str(settings.KeySecDomainFilter)),
		SSRFProtection: l.GetBool(settings.KeySecSSRFProtection, false),
	}
}

// parseList 逗号/空白分隔 → 去空非空片段。
func parseList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseHTTPLike 解析并要求 http(s) URL。
func parseHTTPLike(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("secguard: 仅允许 http(s) URL，收到 %q", raw)
	}
	return u, nil
}

// portOf 显式端口或 scheme 缺省端口。
func portOf(u *url.URL) int {
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}
	if u.Scheme == "https" {
		return 443
	}
	return 80
}

// parsePortList 端口白名单：非法片段跳过（写入侧已校验，读侧容错）。
func parsePortList(raw string) []int {
	var out []int
	for _, p := range parseList(raw) {
		if n, err := strconv.Atoi(p); err == nil && n >= 1 && n <= 65535 {
			out = append(out, n)
		}
	}
	return out
}

// CheckURL 静态校验：scheme/端口白名单/域名白名单/（开启时）DNS 解析级
// 私有地址拦截。返回 nil = 放行。
func CheckURL(ctx context.Context, s Settings, rawURL string) error {
	if !s.SSRFProtection && len(s.AllowPorts) == 0 && len(s.AllowDomains) == 0 {
		return nil // 全关：零开销直通
	}
	u, err := parseHTTPLike(rawURL)
	if err != nil {
		return err
	}
	port := portOf(u)
	if len(s.AllowPorts) > 0 && !containsInt(s.AllowPorts, port) {
		return fmt.Errorf("secguard: 端口 %d 不在允许清单内", port)
	}
	if len(s.AllowDomains) > 0 && !domainAllowed(s.AllowDomains, u.Hostname()) {
		return fmt.Errorf("secguard: 域名 %s 不在允许清单内", u.Hostname())
	}
	if s.SSRFProtection {
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname())
		if err != nil {
			return fmt.Errorf("secguard: 域名解析失败 %s: %w", u.Hostname(), err)
		}
		for _, ip := range ips {
			if isRestricted(ip.IP) && !ipAllowed(ip.IP, s) {
				return fmt.Errorf("secguard: %s 解析到受限地址 %s（SSRF 保护）", u.Hostname(), ip.IP)
			}
		}
	}
	return nil
}

// HTTPClient 返回带拨号级 SSRF 拦截的客户端（Control 钩子在真实 connect
// 前检查对端地址，消除静态解析的 TOCTOU）。保护关闭时原样返回 base。
func HTTPClient(s Settings, base *http.Client) *http.Client {
	if !s.SSRFProtection {
		return base
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: dialControl(s)}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = dialer.DialContext
	client := &http.Client{Transport: tr}
	if base != nil {
		client.Timeout = base.Timeout
		client.CheckRedirect = base.CheckRedirect
	}
	return client
}

// dialControl 返回拨号 Control 钩子：连接前校验对端 IP。
func dialControl(s Settings) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("secguard: 非法地址 %q", address)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("secguard: 非法 IP %q", host)
		}
		if isRestricted(ip) && !ipAllowed(ip, s) {
			return fmt.Errorf("secguard: 拒绝连接受限地址 %s（SSRF 保护）", address)
		}
		return nil
	}
}

// isRestricted 私有/回环/链路本地/未指定地址判定（IPv4+IPv6）。
func isRestricted(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// ipAllowed 命中放行清单（单 IP 或 CIDR）。
func ipAllowed(ip net.IP, s Settings) bool {
	for _, entry := range s.AllowIPs {
		if strings.Contains(entry, "/") {
			if _, cidr, err := net.ParseCIDR(entry); err == nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if parsed := net.ParseIP(entry); parsed != nil && parsed.Equal(ip) {
			return true
		}
	}
	return false
}

// domainAllowed 后缀白名单：host 等于条目或以 "."+条目 结尾（子域匹配，
// 不接受 "evil-example.com" 撞 "example.com" 后缀）。
func domainAllowed(entries []string, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, e := range entries {
		e = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(e), "."))
		if host == e || strings.HasSuffix(host, "."+e) {
			return true
		}
	}
	return false
}

func containsInt(list []int, v int) bool {
	for _, n := range list {
		if n == v {
			return true
		}
	}
	return false
}
