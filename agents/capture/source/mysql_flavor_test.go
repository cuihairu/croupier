package source

import (
	"testing"

	"github.com/go-mysql-org/go-mysql/mysql"
)

// 方言归一（todo 4307）：空兜底 mysql；已知两值原样；未知值拒绝——方言错配
// 的失败形态是连接后 GTID 乱套，须在 Open 前置检查暴露。
func TestNormalizeFlavor(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantEr bool
	}{
		{in: "", want: "mysql"},
		{in: "mysql", want: "mysql"},
		{in: "mariadb", want: "mariadb"},
		{in: "mysql2", wantEr: true},
		{in: "MySQL", wantEr: true}, // 大小写敏感：配置面不做宽容归一
	}
	for _, tt := range tests {
		got, err := MySQLConfig{Flavor: tt.in}.normalizeFlavor()
		if tt.wantEr {
			if err == nil {
				t.Fatalf("normalizeFlavor(%q) = %q, want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeFlavor(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("normalizeFlavor(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// 位点解析按方言分派：mysql=UUID 集合、mariadb=domain-server-seq，互不通用。
func TestParseGTIDSetPerFlavor(t *testing.T) {
	if _, err := mysql.ParseGTIDSet("mariadb", "0-1-5"); err != nil {
		t.Fatalf("mariadb parse: %v", err)
	}
	if _, err := mysql.ParseGTIDSet("mysql", "3E11FA47-71CA-11E1-9E33-C80AA9429562:1-5"); err != nil {
		t.Fatalf("mysql parse: %v", err)
	}
	// 交叉格式必须报错（书签写入方言与源方言不一致时早暴露）。
	if _, err := mysql.ParseGTIDSet("mariadb", "3E11FA47-71CA-11E1-9E33-C80AA9429562:1-5"); err == nil {
		t.Fatal("mysql-format set accepted by mariadb flavor, want error")
	}
	if _, err := mysql.ParseGTIDSet("mysql", "0-1-5"); err == nil {
		t.Fatal("mariadb-format set accepted by mysql flavor, want error")
	}
}
