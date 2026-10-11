package filter

import (
	"fmt"
	"strings"
)

// Result 策略执行结果四分类（过滤策略简档 §5 语义红线：不能一律丢弃）：
//   - SourceFilter 语义：Match=表在授权范围；NoMatch=未授权（默认拒绝）；
//     Error=规则无法执行（解析失败/形态非法）；Unsupported=能力不支持。
//   - Detection 语义（阶段二接线时适用）：NoMatch=条件不满足（正常），
//     Error=规则执行错误（优先保留事件+计数，不静默吞）。
type Result int

const (
	ResultMatch Result = iota
	ResultNoMatch
	ResultError
	ResultUnsupported
)

// String 可观测/日志形态（Stats 分类计数直接用此串作 label）。
func (r Result) String() string {
	switch r {
	case ResultMatch:
		return "match"
	case ResultNoMatch:
		return "no_match"
	case ResultError:
		return "error"
	case ResultUnsupported:
		return "unsupported"
	}
	return "unknown"
}

// isQualifiedTable "库.表" 形态校验（与 gate 包口径一致：一段=缺库前缀，
// 三段以上=表名含点，均拒）。
func isQualifiedTable(s string) bool {
	parts := strings.Split(s, ".")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

// parseTableEntry 解析授权表项：精确 "db.table" 或库级通配 "db.*"。
// 返回 (库, 表, ok)；表段 "*" 表示库级通配。
func parseTableEntry(entry string) (db, table string, ok bool) {
	if !isQualifiedTable(entry) {
		return "", "", false
	}
	i := strings.IndexByte(entry, '.')
	return entry[:i], entry[i+1:], true
}

// TableMatcher 库表授权匹配（SourceFilter 契约面）。
//
// 语义（简档 §1.1/§3 红线，实现即契约）：
//   - 默认拒绝：include 空 = 不授权任何表（授权必须显式；现 source 包
//     「白名单空=全订」是 C1 文件配置兜底语义，策略化接线时以本契约为准，
//     迁移见 todo 阶段三）。
//   - 冲突发布时检测：同一表项同时出现在 include 与 exclude = 构造即错
//     （不靠各 Agent 自行解释，简档 §3）。
//   - 排除优先：多 include 模式共同覆盖、且与某 exclude 项相交时，排除
//     赢（显式 exclude 是运维关闸的手段，不被宽 include 淹没）。
type TableMatcher struct {
	include map[string]tableEntry // "db.table" 或 "db.*" 原样键
	exclude map[string]tableEntry
}

type tableEntry struct {
	db    string
	table string // "*" = 库级通配
}

// NewTableMatcher 构造并校验（include/exclude 冲突在此暴露，非法表项
// 在此暴露——构造失败即发布校验失败，不进运行期）。
func NewTableMatcher(include, exclude []string) (*TableMatcher, error) {
	m := &TableMatcher{
		include: make(map[string]tableEntry, len(include)),
		exclude: make(map[string]tableEntry, len(exclude)),
	}
	for _, entry := range include {
		db, tbl, ok := parseTableEntry(entry)
		if !ok {
			return nil, fmt.Errorf("filter: include entry %q must be \"db.table\" or \"db.*\"", entry)
		}
		m.include[entry] = tableEntry{db: db, table: tbl}
	}
	for _, entry := range exclude {
		db, tbl, ok := parseTableEntry(entry)
		if !ok {
			return nil, fmt.Errorf("filter: exclude entry %q must be \"db.table\" or \"db.*\"", entry)
		}
		// 冲突只认精确项同列两侧：库级 exclude 覆盖 include 是合法的
		// 「关整库」手势（排除确定性取胜，无歧义）；精确表既授又排才是
		// 发布时必须拦下的歧义。
		if tbl != "*" {
			if _, dup := m.include[entry]; dup {
				return nil, fmt.Errorf("filter: table %q listed in both include and exclude", entry)
			}
		}
		m.exclude[entry] = tableEntry{db: db, table: tbl}
	}
	return m, nil
}

// Authorized 表是否在授权采集范围内（SourceFilter 执行面）。
// 排除优先：精确排除与库级排除任一命中即拒（显式 exclude 是运维关闸的
// 手段，不被宽 include 淹没）；授权命中精确项或库级通配项。空 include=
// 全拒（默认拒绝）。
func (m *TableMatcher) Authorized(db, table string) bool {
	if len(m.include) == 0 {
		return false
	}
	qualified := db + "." + table
	if _, excluded := m.exclude[qualified]; excluded {
		return false
	}
	if _, excluded := m.exclude[db+".*"]; excluded {
		return false
	}
	if _, ok := m.include[qualified]; ok {
		return true
	}
	_, ok := m.include[db+".*"]
	return ok
}

// Result 事件级授权判定（Result 四分类口径：授权=Match，未授权=NoMatch）。
func (m *TableMatcher) Result(db, table string) Result {
	if m.Authorized(db, table) {
		return ResultMatch
	}
	return ResultNoMatch
}

// Tables 返回 include 授权集原样键（发布校验/面板回显用；只读遍历面）。
func (m *TableMatcher) Tables() []string {
	out := make([]string, 0, len(m.include))
	for k := range m.include {
		out = append(out, k)
	}
	return out
}

// MissingDependencies 规则依赖检查（发布校验契约：对账双表必须在采集
// 范围内——过滤策略简档 §1.1「不为省事件量漏掉业务流水表」的执行面）。
// 返回未授权表清单；空=依赖满足。nil matcher（未配置 SourceFilter）返回
// 全部依赖——契约层保守：无授权信息时不能声称依赖已满足。
func (m *TableMatcher) MissingDependencies(needed []string) []string {
	var missing []string
	for _, table := range needed {
		if !isQualifiedTable(table) {
			missing = append(missing, table)
			continue
		}
		authorized := false
		if m != nil {
			i := strings.IndexByte(table, '.')
			authorized = m.Authorized(table[:i], table[i+1:])
		}
		if !authorized {
			missing = append(missing, table)
		}
	}
	return missing
}
