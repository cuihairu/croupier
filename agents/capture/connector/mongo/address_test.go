package mongo

import (
	"strings"
	"testing"
)

// AddressFromDSN：host:port 提取（非凭据面）；缺省端口补 27017；多 host 取
// 首个；错误只提变量名。
func TestAddressFromDSN(t *testing.T) {
	if _, err := AddressFromDSN("TEST_MONGO_DSN_UNSET"); err == nil || !strings.Contains(err.Error(), "TEST_MONGO_DSN_UNSET") {
		t.Fatalf("unset env err = %v", err)
	}
	t.Setenv("TEST_MONGO_DSN_ADDR", "mongodb://user:pass@10.0.0.7:27018,10.0.0.8:27019/game?replicaSet=rs0")
	addr, err := AddressFromDSN("TEST_MONGO_DSN_ADDR")
	if err != nil || addr != "10.0.0.7:27018" {
		t.Fatalf("addr = %q err = %v (first host, no credential leak)", addr, err)
	}
	t.Setenv("TEST_MONGO_DSN_ADDR", "mongodb://mongo.internal/game")
	addr, err = AddressFromDSN("TEST_MONGO_DSN_ADDR")
	if err != nil || addr != "mongo.internal:27017" {
		t.Fatalf("addr = %q err = %v (default port)", addr, err)
	}
	t.Setenv("TEST_MONGO_DSN_ADDR", "postgres://db:5432/g")
	if _, err := AddressFromDSN("TEST_MONGO_DSN_ADDR"); err == nil {
		t.Fatal("non-mongodb URI should fail")
	}
}
