package source

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

// ProviderConnector 把既有 Provider 形态源包进统一 SourceConnector 契约——
// todo 4306 增量四「MySQL 双形态收口」：runner 只消费契约面，Provider 形态
// （MySQLSource 进程内直连）只在此边界被桥接，不再向上暴露。
//
// 与 MySQLConnector 的区别：MySQLConnector 走 DSNEnv 凭据通道（明文不进
// 契约），本桥接器服务于 legacy 配置（host/port/user/password 仍在 YAML，
// 凭据留在源实现内部，不进契约、不落日志）。
type ProviderConnector struct {
	sourceType string
	log        *slog.Logger
	src        Provider
	cfg        connector.SourceConfig // Open 时留存（SourceID 透传到契约事件）

	cpMu sync.Mutex
	cp   connector.Position
}

// NewProviderConnector 构造 Provider→契约桥接连接器（不建连接；Poll 时才
// 触发 Provider.Open）。
func NewProviderConnector(src Provider, sourceType string, log *slog.Logger) *ProviderConnector {
	if log == nil {
		log = slog.Default()
	}
	return &ProviderConnector{sourceType: sourceType, log: log, src: src}
}

// Open 契约握手：Provider 形态按 Poll 懒建流，此步只留配置（忽略 DSNEnv——
// legacy 路径凭据不经环境变量通道）。
func (p *ProviderConnector) Open(_ context.Context, cfg connector.SourceConfig) error {
	p.cfg = cfg
	return nil
}

// Discover 桥接形态不支持（legacy 源无信息模式摄取通道；元数据面随契约
// 连接器落地——不静默回空结果）。
func (p *ProviderConnector) Discover(_ context.Context, _ connector.DiscoveryRequest) (*connector.DiscoveryResult, error) {
	return nil, fmt.Errorf("source: discovery unsupported for provider-bridged source %q", p.sourceType)
}

// Snapshot 能力缺失显式拒绝（不静默回空流）。
func (p *ProviderConnector) Snapshot(_ context.Context, _ connector.SnapshotRequest) (<-chan connector.ChangeEvent, error) {
	return nil, connector.ErrUnsupportedSnapshot
}

// Poll 从 req.From.Value 续读（Provider 形态位点是裸串，kind 由源类型决定），
// 事件过契约桥接；req.Tables 非空时二次白名单过滤。
func (p *ProviderConnector) Poll(ctx context.Context, req connector.PollRequest) (<-chan connector.ChangeEvent, error) {
	if p.src == nil {
		return nil, fmt.Errorf("source: no provider bridged")
	}
	events, err := p.src.Open(ctx, Bookmark{Position: req.From.Value})
	if err != nil {
		return nil, err
	}
	out := make(chan connector.ChangeEvent, 64)
	whitelist := map[string]bool{}
	for _, t := range req.Tables {
		whitelist[t] = true
	}
	go func() {
		defer close(out)
		for {
			select {
			case ev, ok := <-events:
				if !ok {
					return
				}
				if len(whitelist) > 0 && !whitelist[ev.Database+"."+ev.Table] {
					continue
				}
				ce, err := bridgeEvent(p.cfg.SourceID, p.sourceType, ev)
				if err != nil {
					p.log.Error("bridge change event", "sourceType", p.sourceType, "error", err)
					return
				}
				select {
				case out <- ce:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// AckCheckpoint records a consumer-confirmed position. Provider internals may
// advance earlier while events are buffered; only this value is resumable.
func (p *ProviderConnector) AckCheckpoint(_ context.Context, position connector.Position) error {
	if p.src == nil {
		return fmt.Errorf("source: no provider bridged")
	}
	if position.Kind != PositionKindOf(p.sourceType) || !position.Valid() {
		return fmt.Errorf("source: invalid %s checkpoint", p.sourceType)
	}
	p.cpMu.Lock()
	defer p.cpMu.Unlock()
	p.cp = position
	return nil
}

// Checkpoint 最近消费确认位点（kind 按源类型：mysql/mariadb=gtid，
// postgres=lsn）。未确认前返回零位点，避免把已读取但未消费的位置持久化。
func (p *ProviderConnector) Checkpoint(context.Context) (connector.Position, error) {
	if p.src == nil {
		return connector.Position{}, fmt.Errorf("source: no provider bridged")
	}
	p.cpMu.Lock()
	defer p.cpMu.Unlock()
	return p.cp, nil
}

// Close 断开源流（幂等）。
func (p *ProviderConnector) Close(_ context.Context) error {
	if p.src == nil {
		return nil
	}
	return p.src.Close()
}

// bridgeEvent 进程内形态 → 契约形态（无损映射）：行像按位置下标键化（MySQL
// binlog ROW 裸数组，契约 ColumnsMapped=false 语义）、位点 kind 按源类型、
// 操作闭集互转（未知操作显式报错，不静默映射）。sourceID 空时回落源类型
// （契约事件自检要求身份非空）。
func bridgeEvent(sourceID, sourceType string, ev ChangeEvent) (connector.ChangeEvent, error) {
	if sourceID == "" {
		sourceID = sourceType
	}
	op, err := connectorOperation(ev.Op)
	if err != nil {
		return connector.ChangeEvent{}, err
	}
	return connector.ChangeEvent{
		SourceID:      sourceID,
		SourceType:    sourceType,
		Database:      ev.Database,
		Table:         ev.Table,
		Operation:     op,
		Before:        positionImage(ev.BeforeJSON),
		After:         positionImage(ev.AfterJSON),
		ColumnsMapped: false, // binlog ROW 裸下标；列名映射随 Discover 摄取
		Account:       ev.Account,
		TsUnixMs:      ev.TsUnixMs,
		Position:      connector.Position{Kind: PositionKindOf(sourceType), Value: ev.GTID},
	}, nil
}

// PositionKindOf 源类型的位点 kind（checkpoint 位点还原用：kind 由源类型决定，
// 位点值本身不带 kind 前缀）。
func PositionKindOf(sourceType string) string {
	switch sourceType {
	case "postgres":
		return "lsn"
	case "mongodb":
		return "token"
	default:
		return "gtid"
	}
}

// connectorOperation 源层操作 → 契约操作（闭集互转）。
func connectorOperation(op Op) (connector.Operation, error) {
	switch op {
	case OpInsert:
		return connector.OpInsert, nil
	case OpUpdate:
		return connector.OpUpdate, nil
	case OpDelete:
		return connector.OpDelete, nil
	default:
		return "", fmt.Errorf("source: unknown change op %q", op)
	}
}

// positionImage 行像 JSON 数组 → 位置下标键 map（{"0","1",…}）。空行像/
// 非法 JSON 返回 nil（与 source 层的 nil 语义对齐）。
func positionImage(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var vals []json.RawMessage
	if err := json.Unmarshal(raw, &vals); err != nil {
		return nil
	}
	m := make(map[string]json.RawMessage, len(vals))
	for i, v := range vals {
		m[strconv.Itoa(i)] = v
	}
	return m
}
