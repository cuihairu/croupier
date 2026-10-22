package source

import (
	"strings"
	"testing"
)

// AddressFromDSN Readiness 探测地址解析：host:port 非凭据面；unix socket 不可
// TCP 探测 → 空串降级；错误消息只提环境变量名（凭据红线）。
func TestAddressFromDSN(t *testing.T) {
	const env = "CROUPIER_CAPTURE_MYSQL_ADDR_TEST"
	t.Setenv(env, "probe_user:probe_pass@tcp(cdc-host:3307)/db")
	addr, err := AddressFromDSN(env)
	if err != nil || addr != "cdc-host:3307" {
		t.Fatalf("addr = %q, %v", addr, err)
	}
	t.Setenv(env, "probe_user:probe_pass@tcp(cdc-host)/db")
	if addr, err = AddressFromDSN(env); err != nil || addr != "cdc-host:3306" {
		t.Fatalf("default port addr = %q, %v", addr, err)
	}
	t.Setenv(env, "probe_user:probe_pass@unix(/tmp/mysql.sock)/db")
	if addr, err = AddressFromDSN(env); err != nil || addr != "" {
		t.Fatalf("unix addr = %q, %v (want empty, nil)", addr, err)
	}
	if _, err := AddressFromDSN(env + "_MISSING"); err == nil || !strings.Contains(err.Error(), env+"_MISSING") {
		t.Fatalf("missing env error = %v", err)
	}
	t.Setenv(env, "bad@@dsn")
	if _, err := AddressFromDSN(env); err == nil {
		t.Fatal("malformed dsn accepted")
	}
}
