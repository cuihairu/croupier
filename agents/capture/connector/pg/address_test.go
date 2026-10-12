package pg

import (
	"strings"
	"testing"
)

// AddressFromDSN Readiness 探测地址解析（非凭据面）；错误消息只提环境变量名
// （凭据红线：不回显取值）。
func TestAddressFromDSN(t *testing.T) {
	const env = "CROUPIER_CAPTURE_PG_ADDR_TEST"
	t.Setenv(env, "postgres://repl:secret@cdc-host:5433/db")
	addr, err := AddressFromDSN(env)
	if err != nil || addr != "cdc-host:5433" {
		t.Fatalf("addr = %q, %v", addr, err)
	}
	t.Setenv(env, "postgres://repl:secret@cdc-host/db")
	if addr, err = AddressFromDSN(env); err != nil || addr != "cdc-host:5432" {
		t.Fatalf("default port addr = %q, %v", addr, err)
	}
	if _, err := AddressFromDSN(env + "_MISSING"); err == nil || !strings.Contains(err.Error(), env+"_MISSING") {
		t.Fatalf("missing env error = %v", err)
	}
	t.Setenv(env, "not a dsn at all")
	if _, err := AddressFromDSN(env); err == nil {
		t.Fatal("malformed dsn accepted")
	}
}
