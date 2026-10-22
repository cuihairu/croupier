package source

import (
	"context"
	"encoding/json"
	"fmt"
)

// Provider 事件源 Provider 接口（设计 §3.2）：把源日志翻译成 ChangeEvent 流。
// 实现方负责断线重连与位点推进；事件经 MatchesTable 过滤后才进 channel。
type Provider interface {
	// Open 建立 CDC 连接并从 bookmark 起点续读。返回的事件 channel 在 ctx
	// 取消或致命错误后关闭。
	Open(ctx context.Context, bookmark Bookmark) (<-chan ChangeEvent, error)
	// Bookmark 当前位点快照（崩溃前由 runner 周期落盘）。
	Bookmark() Bookmark
	Close() error
}

// Bookmark 位点书签（GTID/LSN bookmark，崩溃安全）：本地原子持久化 + 上报
// server 双写（上报侧随 C2 wire 落地），崩溃拉起后从 bookmark 续读——不丢
// 事件、幂等去重。
type Bookmark struct {
	// GTIDSet MySQL GTID 集合形态（如 "3E11FA47-71CA-11E1-9E33-C80AA9429562:1-5"）；
	// PG 侧为 LSN 字符串，同样存此字段。
	Position string `json:"position"`
	// UpdatedAtUnixMs 最近推进时刻（运维排查位点滞后告警用）。
	UpdatedAtUnixMs int64 `json:"updatedAtUnixMs"`
}

// Valid 书签是否可用（空位点 = 从可得起点重扫，binlog 保留期短于停机时长
// 时的降级路径，设计 §9）。
func (b Bookmark) Valid() bool { return b.Position != "" }

// Store 位点持久化（本地文件，原子写 tmp+rename）。
type Store struct {
	path string
}

// NewStore 位点文件存取器。
func NewStore(path string) *Store { return &Store{path: path} }

// Load 读回书签；文件不存在返回零值书签（首启/位点丢失降级重扫）。
func (s *Store) Load() (Bookmark, error) {
	data, err := readFile(s.path)
	if err != nil {
		return Bookmark{}, nil // 不存在/不可读 = 空书签，不阻断首启
	}
	var b Bookmark
	if err := json.Unmarshal(data, &b); err != nil {
		return Bookmark{}, fmt.Errorf("source: corrupt bookmark %s: %w", s.path, err)
	}
	return b, nil
}

// Save 原子落盘书签（写同目录临时文件后 rename 覆盖——崩溃时要么旧位点要么
// 新位点，不出现半截文件）。
func (s *Store) Save(b Bookmark) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return atomicWrite(s.path, data)
}
