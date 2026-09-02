// 覆盖目标：internal/platform/tlsutil 仅含 ClientTLSConfig 结构体定义，
// 无任何可执行语句（go tool cover 报告 [no statements]）。此处补充
// 结构体字段构造/零值用例，锁定契约防止字段被意外删改。
package tlsutil

import "testing"

func TestClientTLSConfig_Fields(t *testing.T) {
	cfg := ClientTLSConfig{
		CertFile:           "/etc/certs/server.crt",
		KeyFile:            "/etc/certs/server.key",
		CAFile:             "/etc/certs/ca.crt",
		ServerName:         "croupier-server",
		InsecureSkipVerify: true,
	}

	if cfg.CertFile != "/etc/certs/server.crt" {
		t.Fatalf("CertFile = %q", cfg.CertFile)
	}
	if cfg.KeyFile != "/etc/certs/server.key" {
		t.Fatalf("KeyFile = %q", cfg.KeyFile)
	}
	if cfg.CAFile != "/etc/certs/ca.crt" {
		t.Fatalf("CAFile = %q", cfg.CAFile)
	}
	if cfg.ServerName != "croupier-server" {
		t.Fatalf("ServerName = %q", cfg.ServerName)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify should be true")
	}
}

func TestClientTLSConfig_ZeroValue(t *testing.T) {
	var cfg ClientTLSConfig
	if cfg.CertFile != "" || cfg.KeyFile != "" || cfg.CAFile != "" || cfg.ServerName != "" {
		t.Fatal("zero value should have empty string fields")
	}
	if cfg.InsecureSkipVerify {
		t.Fatal("zero value InsecureSkipVerify should default to false (安全默认)")
	}
}
