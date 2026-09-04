// Package executionlog 提供执行留痕（R1）的异步写入器：payload 级
// 请求/响应落库 execution_logs，供事后审查。
//
// 写入约束（fail-open）：
//   - 调用方 Log() 永不阻塞：队列满即丢弃并计数（丢弃只告警，不影响执行主路径）
//   - 后台 goroutine 批量落库；Stop 时冲刷队列
//   - 写库失败仅记日志，不重试不阻塞
package executionlog

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

const (
	SourceInvoke = "invoke"
	SourcePage   = "page"

	StatusOK   = "ok"
	StatusFail = "error"

	DefaultMaxPayloadBytes = 64 * 1024
	defaultQueueSize       = 4096
	defaultBatchSize       = 200
	defaultFlushInterval   = 2 * time.Second
)

// Entry 单次执行的留痕载荷。
type Entry struct {
	GameID     string
	Env        string
	Source     string
	FunctionID string
	PageKey    string
	BindingID  string
	Actor      string
	Route      string
	Status     string
	DurationMs int64
	TraceID    string
	// Request/Response 为任意 JSON 值；写入前统一脱敏并按上限截断。
	Request  interface{}
	Response interface{}
}

type Config struct {
	Enabled         bool
	MaxPayloadBytes int
	QueueSize       int
	BatchSize       int
	FlushInterval   time.Duration
}

// Writer 异步批量写 execution_logs。
type Writer struct {
	model     *model.ExecutionLogModel
	ch        chan Entry
	batchSize int
	flush     time.Duration
	maxBytes  int
	dropped   atomic.Int64
	wg        sync.WaitGroup
	stopOnce  sync.Once
	stopCh    chan struct{}
}

func NewWriter(db *gorm.DB, cfg Config) *Writer {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = defaultFlushInterval
	}
	if cfg.MaxPayloadBytes <= 0 {
		cfg.MaxPayloadBytes = DefaultMaxPayloadBytes
	}
	return &Writer{
		model:     model.NewExecutionLogModel(db),
		ch:        make(chan Entry, cfg.QueueSize),
		batchSize: cfg.BatchSize,
		flush:     cfg.FlushInterval,
		maxBytes:  cfg.MaxPayloadBytes,
		stopCh:    make(chan struct{}),
	}
}

// Log 非阻塞投递；队列满丢弃并计数。enabled=false 时由调用方短路。
func (w *Writer) Log(entry Entry) {
	if w == nil {
		return
	}
	select {
	case w.ch <- entry:
	default:
		w.dropped.Add(1)
	}
}

// Dropped 返回累计丢弃条数（监控/自检用）。
func (w *Writer) Dropped() int64 {
	if w == nil {
		return 0
	}
	return w.dropped.Load()
}

// Run 消费队列直到 Stop。
func (w *Writer) Run(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		batch := make([]model.ExecutionLog, 0, w.batchSize)
		flush := func() {
			if len(batch) == 0 {
				return
			}
			if err := w.model.CreateBatch(ctx, batch); err != nil {
				slog.WarnContext(ctx, "execution log batch write failed", "count", len(batch), "error", err)
			}
			batch = batch[:0]
		}
		ticker := time.NewTicker(w.flush)
		defer ticker.Stop()
		for {
			select {
			case entry := <-w.ch:
				batch = append(batch, w.toModel(entry))
				if len(batch) >= w.batchSize {
					flush()
				}
			case <-ticker.C:
				flush()
			case <-w.stopCh:
				// 冲刷剩余
				for {
					select {
					case entry := <-w.ch:
						batch = append(batch, w.toModel(entry))
						if len(batch) >= w.batchSize {
							flush()
						}
					default:
						flush()
						return
					}
				}
			}
		}
	}()
}

// Stop 停止消费并冲刷队列；可安全重复调用。
func (w *Writer) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
}

func (w *Writer) toModel(entry Entry) model.ExecutionLog {
	return model.ExecutionLog{
		GameID:         entry.GameID,
		Env:            entry.Env,
		Source:         entry.Source,
		FunctionID:     entry.FunctionID,
		PageKey:        entry.PageKey,
		BindingID:      entry.BindingID,
		Actor:          entry.Actor,
		Route:          entry.Route,
		Status:         entry.Status,
		DurationMs:     entry.DurationMs,
		TraceID:        entry.TraceID,
		RequestPayload: encodePayload(entry.Request, w.maxBytes),
		ResponseBody:   encodePayload(entry.Response, w.maxBytes),
		Truncated:      exceedsBytes(entry.Request, w.maxBytes) || exceedsBytes(entry.Response, w.maxBytes),
		CreatedAt:      time.Now().UTC(),
	}
}

// encodePayload 脱敏后序列化；超出上限截断为 JSON 字符串摘要（保留截断标记）。
func encodePayload(value interface{}, maxBytes int) model.JSON {
	if value == nil {
		return model.JSON("null")
	}
	masked := audit.MaskSensitiveValue(normalizeJSON(value))
	raw, err := json.Marshal(masked)
	if err != nil {
		return model.JSON(`{"logEncodeError":true}`)
	}
	if len(raw) > maxBytes {
		return model.JSON(`{"logTruncated":true,"bytes":` + strconv.Itoa(len(raw)) + `}`)
	}
	return model.JSON(raw)
}

func exceedsBytes(value interface{}, maxBytes int) bool {
	if value == nil {
		return false
	}
	raw, err := json.Marshal(audit.MaskSensitiveValue(normalizeJSON(value)))
	if err != nil {
		return false
	}
	return len(raw) > maxBytes
}

// normalizeJSON 把任意值经 JSON 往返转成 map/slice 标量形态，保证
// maskMap 的递归脱敏能命中嵌套字段；不可序列化值降级为其字符串形式。
func normalizeJSON(value interface{}) interface{} {
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]interface{}{"logUnserializable": err.Error()}
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]interface{}{"logUnserializable": err.Error()}
	}
	return out
}
