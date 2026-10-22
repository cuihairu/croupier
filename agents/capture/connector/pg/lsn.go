// Package pg PostgreSQL logical replication 解码层（CDC 平台化批 4306 P0 第一
// 增量）：pgoutput 协议二进制负载 → connector.ChangeEvent（平台契约面）。
//
// 定位：纯逻辑零驱动——本包不建立网络连接、不引 pgx/pglogrepl；复制流握手
// （CREATE_REPLICATION_SLOT/START_REPLICATION/keepalive）是下一增量（流式接线
// 层），活体验证随 4301 式清单走。单元测试用手工二进制 fixture 锁协议布局。
//
// 协议依据：PostgreSQL 文档「Logical Replication Message Formats」（pgoutput,
// protocol_version=1）。事务事件在 Commit 点一次吐出（事务完整性：从 commit
// 位点恢复时整事务原子重放）。
package pg

import (
	"fmt"
	"strconv"
	"strings"
)

// LSN PostgreSQL WAL 位点（Log Sequence Number，uint64）。标准串形态 "X/Y"
// （两段十六进制，如 "16/3C219F48"）；checkpoint 持久化形态用十进制串
// （connector.Position.Value 契约注释约定 "LSN 十进制"）。
type LSN uint64

// ParseLSN 解析 LSN 串：接受 "X/Y" 十六进制标准形态与纯十进制串（后者是
// 本包 Decimal() 的输出，bookmark 回放路径）。其他形态报错。
func ParseLSN(s string) (LSN, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("pg: empty lsn")
	}
	if hi, lo, ok := strings.Cut(s, "/"); ok {
		h, err := strconv.ParseUint(hi, 16, 32)
		if err != nil {
			return 0, fmt.Errorf("pg: malformed lsn %q: %w", s, err)
		}
		l, err := strconv.ParseUint(lo, 16, 32)
		if err != nil {
			return 0, fmt.Errorf("pg: malformed lsn %q: %w", s, err)
		}
		return LSN(h<<32 | l), nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("pg: malformed lsn %q: %w", s, err)
	}
	return LSN(v), nil
}

// String 标准展示形态 "X/Y"（十六进制两段，PG psql 口径）。
func (l LSN) String() string {
	return fmt.Sprintf("%X/%X", uint64(l)>>32, uint64(l)&0xFFFFFFFF)
}

// Decimal 十进制串（connector.Position.Value 持久化形态；ParseLSN 可逆）。
func (l LSN) Decimal() string {
	return strconv.FormatUint(uint64(l), 10)
}
