package pg

import (
	"strings"
	"testing"
)

// pgoutput 解析器的防御臂收口：各消息的长度守卫、cstring 未终止、列数
// 超载、元组各阶段截断、K/O/N 标记路径的错误包装，以及 tupleSize 的
// 坏负载归零兜底。
//
// 诚实跳过：无——本文件全部臂均为可达的错误路径（坏负载直接构造）。

func TestParseRelationMalformedArms(t *testing.T) {
	// schema cstring 未终止（64-65 → 79-80 包装）
	p := append(u32v(1), []byte("abcdefgh")...) // 12B，无 NUL
	if _, err := ParseRelation(p); err == nil {
		t.Fatal("unterminated schema accepted")
	} else if !contains(err, "relation schema") {
		t.Fatalf("schema err = %v", err)
	}

	// name cstring 未终止（82-83）
	p = append(append(u32v(1), cstr("s")...), []byte("abcde")...)
	if _, err := ParseRelation(p); err == nil || !contains(err, "relation name") {
		t.Fatalf("name err = %v", err)
	}

	// 列数超载防御（89-90）
	p = append(u32v(1), cstr("s")...)
	p = append(p, cstr("t")...)
	p = append(p, 'd')
	p = append(p, u16v(0xFFFF)...)
	if _, err := ParseRelation(p); err == nil || !contains(err, "column count 65535 exceeds payload") {
		t.Fatalf("colcount err = %v", err)
	}

	// 列元数据截断（95-96）
	p = append(u32v(1), cstr("s")...)
	p = append(p, cstr("t")...)
	p = append(p, 'd')
	p = append(p, u16v(1)...)
	if _, err := ParseRelation(p); err == nil || !contains(err, "column 0 truncated") {
		t.Fatalf("col trunc err = %v", err)
	}

	// 列名 cstring 未终止（100-101）：需 ≥6 字节（过 off+6 截断守卫）且无 NUL
	p = append(u32v(1), cstr("s")...)
	p = append(p, cstr("t")...)
	p = append(p, 'd')
	p = append(p, u16v(1)...)
	p = append(p, 0, 'a', 'b', 'c', 'd', 'e') // flags + 5 字节未终止名
	if _, err := ParseRelation(p); err == nil || !contains(err, "column 0 name") {
		t.Fatalf("col name err = %v", err)
	}
}

func TestParseTupleMalformedArms(t *testing.T) {
	// 负载不足 2 字节（128-129）
	for _, p := range [][]byte{nil, {0}} {
		if _, err := ParseTuple(p); err == nil || !contains(err, "tuple payload too short") {
			t.Fatalf("short tuple err = %v (payload %v)", err, p)
		}
	}

	// 文本值长度头截断（144-145）
	if _, err := ParseTuple(append(u16v(1), 't')); err == nil || !contains(err, "column 0 length truncated") {
		t.Fatalf("len trunc err = %v", err)
	}

	// 文本值数据截断（149-150）
	p := append(u16v(1), 't')
	p = append(p, u32v(10)...)
	p = append(p, 'a', 'b')
	if _, err := ParseTuple(p); err == nil || !contains(err, "column 0 data truncated") {
		t.Fatalf("data trunc err = %v", err)
	}
}

func TestParseInsertArms(t *testing.T) {
	// 负载不足 5 字节（207-208）
	if _, err := ParseInsert([]byte("abc")); err == nil || !contains(err, "insert payload too short") {
		t.Fatalf("short insert err = %v", err)
	}

	// 新元组解析失败包装（215-216）
	if _, err := ParseInsert(append(append(u32v(1), 'N'), 0)); err == nil || !contains(err, "insert tuple") {
		t.Fatalf("tuple err = %v", err)
	}
}

func TestParseUpdateArms(t *testing.T) {
	// 负载不足 5 字节（234-235）
	if _, err := ParseUpdate([]byte("abc")); err == nil || !contains(err, "update payload too short") {
		t.Fatalf("short update err = %v", err)
	}

	// 'K' 键元组解析失败（244-245）：1 字节负载不足 2 字节头
	if _, err := ParseUpdate(append(u32v(1), 'K', 0)); err == nil || !contains(err, "update key tuple") {
		t.Fatalf("key tuple err = %v", err)
	}

	// 'O' 旧元组解析失败（256-257）
	if _, err := ParseUpdate(append(u32v(1), 'O', 0)); err == nil || !contains(err, "update old tuple") {
		t.Fatalf("old tuple err = %v", err)
	}

	// 'O' 后缺新元组 'N' 标记（261-262）
	noN := append(u32v(1), 'O')
	noN = append(noN, u16v(0)...)
	if _, err := ParseUpdate(noN); err == nil || !contains(err, "missing new-tuple") {
		t.Fatalf("missing N err = %v", err)
	}

	// 新元组解析失败（271-272）
	if _, err := ParseUpdate(append(append(u32v(1), 'N'), 0)); err == nil || !contains(err, "update new tuple") {
		t.Fatalf("new tuple err = %v", err)
	}
}

func TestParseDeleteArms(t *testing.T) {
	// 负载不足 6 字节（305-306）
	if _, err := ParseDelete([]byte("abcde")); err == nil || !contains(err, "delete payload too short") {
		t.Fatalf("short delete err = %v", err)
	}

	// 旧元组解析失败（317-318）
	if _, err := ParseDelete(append(u32v(1), 'K', 0)); err == nil || !contains(err, "delete old tuple") {
		t.Fatalf("old tuple err = %v", err)
	}
}

func TestParseTruncateShort(t *testing.T) {
	if _, err := ParseTruncate([]byte("abcd")); err == nil || !contains(err, "truncate payload too short") {
		t.Fatalf("short truncate err = %v", err)
	}
}

func TestTupleSizeMalformedReturnsZero(t *testing.T) {
	// 坏负载 → ParseTuple 失败 → 归零兜底（282-283）
	if got := tupleSize([]byte{0, 1}); got != 0 {
		t.Fatalf("tupleSize(bad) = %d, want 0", got)
	}
	// 合法单文本列 → 2 头 + 1 kind + 4 len + n
	if got := tupleSize(tupleData(textDatum("hi"))); got != 2+1+4+2 {
		t.Fatalf("tupleSize = %d, want %d", got, 2+1+4+2)
	}
}

// contains 错误串包含断言（tests 侧共用的小工具）。
func contains(err error, sub string) bool {
	return err != nil && strings.Contains(err.Error(), sub)
}
