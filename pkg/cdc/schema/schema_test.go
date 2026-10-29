package schema

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// lsnLessForTest 与 PG 连接器同口径的十进制位点序（长度=量级）。
func lsnLessForTest(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

func testSchema(db, table string, cols ...string) TableSchema {
	ts := TableSchema{Database: db, Table: table}
	for _, name := range cols {
		ts.Columns = append(ts.Columns, Column{Name: name, Type: "23"})
	}
	return ts
}

func TestHistoryRecordAndAt(t *testing.T) {
	h := NewHistory(lsnLessForTest)
	v1 := testSchema("game", "items", "id", "name")
	v2 := testSchema("game", "items", "id", "name", "gold")

	if err := h.Record("game.items", 1, "100", v1); err != nil {
		t.Fatalf("record v1: %v", err)
	}
	if err := h.Record("game.items", 2, "500", v2); err != nil {
		t.Fatalf("record v2: %v", err)
	}

	// 时点查询：位点前的版本归前一版；首版之前无版本（未知→found=false）。
	cases := []struct {
		pos      string
		revision int64
		found    bool
	}{
		{"50", 0, false},  // 早于首版：无版本（不猜）
		{"100", 1, true},  // 恰在 v1 生效点
		{"499", 1, true},  // v2 之前
		{"500", 2, true},  // 恰在 v2 生效点
		{"9999", 2, true}, // 之后
	}
	for _, c := range cases {
		ts, rev, found := h.At("game.items", c.pos)
		if found != c.found || rev != c.revision {
			t.Fatalf("At(%q) = rev %d found %v, want rev %d found %v", c.pos, rev, found, c.revision, c.found)
		}
		if found && !ts.HasColumn("id") {
			t.Fatalf("At(%q) schema missing key column", c.pos)
		}
	}
	// v2 时点含新列，v1 时点不含——列集按时点区分。
	tsV2, _, _ := h.At("game.items", "500")
	if !tsV2.HasColumn("gold") {
		t.Fatal("v2 schema missing new column gold")
	}
	tsV1, _, _ := h.At("game.items", "300")
	if tsV1.HasColumn("gold") {
		t.Fatal("v1 schema must not contain gold (added later)")
	}
}

func TestHistoryRecordIdempotentReplay(t *testing.T) {
	h := NewHistory(lsnLessForTest)
	v1 := testSchema("game", "items", "id")
	if err := h.Record("game.items", 1, "100", v1); err != nil {
		t.Fatalf("first: %v", err)
	}
	// 重连后同版本同位点同结构重发：幂等吸收。
	if err := h.Record("game.items", 1, "100", v1); err != nil {
		t.Fatalf("replay same version: %v", err)
	}
	if got := len(h.Versions("game.items")); got != 1 {
		t.Fatalf("versions = %d, want 1", got)
	}
}

func TestHistoryRecordRejectsBadInput(t *testing.T) {
	h := NewHistory(lsnLessForTest)
	if err := h.Record("game.items", 1, "100", testSchema("game", "items", "id")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cases := []struct {
		name      string
		qualified string
		revision  int64
		from      string
		ts        TableSchema
		wantIn    string
	}{
		{"out of order", "game.items", 2, "100", testSchema("game", "items", "id", "a"), "not after"},
		{"backwards", "game.items", 2, "50", testSchema("game", "items", "id", "a"), "not after"},
		{"same revision conflict", "game.items", 1, "200", testSchema("game", "items", "id", "a"), "conflicting"},
		{"zero revision", "game.items", 0, "200", testSchema("game", "items", "id"), "positive"},
		{"empty position", "game.items", 2, "", testSchema("game", "items", "id"), "no source position"},
		{"table key mismatch", "game.items", 2, "200", testSchema("game", "other", "id"), "does not match"},
		{"no columns", "game.items", 2, "200", TableSchema{Database: "game", Table: "items"}, "no columns"},
	}
	for _, c := range cases {
		err := h.Record(c.qualified, c.revision, c.from, c.ts)
		if err == nil || !strings.Contains(err.Error(), c.wantIn) {
			t.Fatalf("%s: err = %v, want containing %q", c.name, err, c.wantIn)
		}
	}
	// 拒绝后历史不被污染。
	if got := len(h.Versions("game.items")); got != 1 {
		t.Fatalf("versions after rejects = %d, want 1", got)
	}
}

func TestHistoryUnknownTable(t *testing.T) {
	h := NewHistory(lsnLessForTest)
	if err := h.Record("game.items", 1, "100", testSchema("game", "items", "id")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, _, found := h.At("game.other", "100"); found {
		t.Fatal("unknown table must not be found")
	}
	if tables := h.Tables(); len(tables) != 1 || tables[0] != "game.items" {
		t.Fatalf("tables = %v", tables)
	}
}

func TestValidateColumnsTimePointCheck(t *testing.T) {
	old := testSchema("game", "items", "id", "name")
	// 旧事件列集 ⊆ 旧 schema：通过（旧事件按事务时点校验）。
	if err := ValidateColumns(old, []string{"id", "name"}); err != nil {
		t.Fatalf("old event under old schema: %v", err)
	}
	// 部分列（副本身份旧像）合法。
	if err := ValidateColumns(old, []string{"id"}); err != nil {
		t.Fatalf("partial image: %v", err)
	}
	// 引用时点尚不存在的新列（gold 由后续 DDL 加入）：拒绝——不静默重映射。
	err := ValidateColumns(old, []string{"id", "gold"})
	if err == nil || !strings.Contains(err.Error(), "gold") {
		t.Fatalf("future column err = %v, want naming gold", err)
	}
	// 空列集拒绝（无可校验面）。
	if err := ValidateColumns(old, nil); err == nil {
		t.Fatal("empty columns must be rejected")
	}
}

func TestHistoryValidateAndClone(t *testing.T) {
	h := NewHistory(lsnLessForTest)
	if err := h.Record("a.t", 1, "10", testSchema("a", "t", "id")); err != nil {
		t.Fatal(err)
	}
	if err := h.Record("a.t", 2, "20", testSchema("a", "t", "id", "x")); err != nil {
		t.Fatal(err)
	}
	if err := h.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	cp := h.Clone()
	if _, _, found := cp.At("a.t", "15"); !found {
		t.Fatal("clone lost versions")
	}
	// clone 后续记录不影响原历史（深隔离）。
	if err := cp.Record("b.u", 1, "10", testSchema("b", "u", "id")); err != nil {
		t.Fatal(err)
	}
	if got := h.Tables(); len(got) != 1 {
		t.Fatalf("original history polluted: %v", got)
	}
}

// TestHistoryConcurrentRecordAndClone exercises the connector's usage shape:
// the pump goroutine Records while a recovery query Clones and reads. Run with
// -race to prove there is no concurrent map iteration + map write.
func TestHistoryConcurrentRecordAndClone(t *testing.T) {
	h := NewHistory(lsnLessForTest)
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		rev := int64(0)
		for {
			select {
			case <-stop:
				return
			default:
			}
			rev++
			table := fmt.Sprintf("t%d", rev%8)
			qualified := "game." + table
			if err := h.Record(qualified, rev, fmt.Sprintf("%d", rev*100), testSchema("game", table, "id", "name")); err != nil {
				t.Errorf("record: %v", err)
				return
			}
		}
	}()

	for i := 0; i < 200; i++ {
		cp := h.Clone()
		cp.At("game.t0", "1")
		cp.Tables()
	}
	close(stop)
	wg.Wait()
}

func TestHistoryConcurrentRecordAndRead(t *testing.T) {
	h := NewHistory(lsnLessForTest)
	const writers, perWriter = 4, 200

	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			for i := 0; i < perWriter; i++ {
				table := fmt.Sprintf("g.t%d", w*perWriter+i)
				pos := fmt.Sprintf("%d", 100+i)
				_ = h.Record(table, int64(i+1), pos, testSchema("g", fmt.Sprintf("t%d", w*perWriter+i), "id"))
			}
		}(w)
	}
	for r := 0; r < writers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < perWriter; i++ {
				cp := h.Clone()
				_ = cp.Validate()
				_, _, _ = h.At("g.t0", "150")
				_ = h.Versions("g.t0")
				_ = h.Tables()
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := len(h.Tables()); got != writers*perWriter {
		t.Fatalf("tables = %d, want %d", got, writers*perWriter)
	}
}

func TestQualifiedOf(t *testing.T) {
	if db, tbl, ok := QualifiedOf("game.items"); !ok || db != "game" || tbl != "items" {
		t.Fatalf("QualifiedOf(game.items) = %q %q %v", db, tbl, ok)
	}
	for _, bad := range []string{"items", "", ".items", "game.", "a.b.c"} {
		if _, _, ok := QualifiedOf(bad); ok {
			t.Fatalf("QualifiedOf(%q) must fail", bad)
		}
	}
}
