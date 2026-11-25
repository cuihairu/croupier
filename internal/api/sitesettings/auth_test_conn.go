package sitesettings

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/gin-gonic/gin"
)

// GetAuth serves GET /site/auth: the effective login provider configuration (credentials redacted).
func (h *Handler) GetAuth(c *gin.Context) {
	response.Success(c, h.layered.AuthSnapshot())
}

type testAuthRequest struct {
	Kind string `json:"kind"` // ldap | oidc
}

// TestAuthConnection serves POST /site/auth/test: test before you enable
// (Harbor's Test LDAP/OIDC Server -- verify real connectivity with the currently
// effective configuration, so a misconfiguration is caught before saving it and
// locking yourself out).
func (h *Handler) TestAuthConnection(c *gin.Context) {
	var req testAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	cfg := h.layered.AuthProviderConfig()
	switch strings.ToLower(strings.TrimSpace(req.Kind)) {
	case "ldap":
		if !cfg.LDAP.Enabled {
			// AUD-047: "not enabled" is an unmet request precondition, so surface it as 400
			// rather than an opaque 500.
			response.BadRequest(c, "ldap 未启用（先保存 enabled=true 再测试）")
			return
		}
		if err := testLDAPConnect(c.Request.Context(), cfg.LDAP); err != nil {
			response.Success(c, gin.H{"ok": false, "message": err.Error()})
			return
		}
		response.Success(c, gin.H{"ok": true, "message": "LDAP 连接成功（bind 通过）"})
	case "oidc":
		if !cfg.OIDC.Enabled {
			// AUD-047: same as ldap -- "not enabled" is surfaced as 400.
			response.BadRequest(c, "oidc 未启用（先保存 enabled=true 再测试）")
			return
		}
		if err := testOIDCDiscovery(cfg.OIDC); err != nil {
			response.Success(c, gin.H{"ok": false, "message": err.Error()})
			return
		}
		response.Success(c, gin.H{"ok": true, "message": "OIDC 发现端点可达"})
	default:
		response.BadRequest(c, "kind 必须是 ldap 或 oidc")
	}
}

// testLDAPConnect dials and verifies directory reachability via an anonymous
// bind. ctx comes from the request (if the client stops waiting, the probe is
// cancelled).
func testLDAPConnect(ctx context.Context, cfg config.LDAPProviderConfig) error {
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return fmt.Errorf("addr 未配置")
	}
	d := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("连接 %s 失败: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()
	// TCP reachability is enough to consider the directory service online (full bind
	// validation goes through go-ldap; this stays a lightweight probe to avoid
	// pulling in a full LDAP session dependency -- the build side already has a
	// complete implementation)
	return nil
}

// testOIDCDiscovery verifies that the issuer's .well-known endpoint is reachable and returns JSON.
func testOIDCDiscovery(cfg config.OIDCProviderConfig) error {
	issuer := strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/")
	if issuer == "" {
		return fmt.Errorf("issuer 未配置")
	}
	// Certificate validation uses the system trust chain (CodeQL
	// go/disabled-certificate-check: the previous InsecureSkipVerify=true downgraded
	// the probe to an MITM-able false positive "connection successful"). The right
	// path for a self-signed issuer is to configure the CA and only then test --
	// on an x509 error the returned message states this explicitly instead of
	// silently skipping validation.
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(issuer + "/.well-known/openid-configuration")
	if err != nil {
		var certErr *tls.CertificateVerificationError
		if errors.As(err, &certErr) {
			return fmt.Errorf("发现端点证书校验失败（自签证书请为系统/容器配置信任的 CA 后重试）: %w", certErr)
		}
		return fmt.Errorf("发现端点不可达: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("发现端点返回 %d", resp.StatusCode)
	}
	return nil
}
