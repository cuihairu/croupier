package registry

import (
	"context"
	"encoding/json"

	"time"

	"github.com/cuihairu/croupier/internal/common/dbtype"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// Add adds a metrics report to the store.
func (s *MetricsStore) Add(agentID string, report *opsv1.MetricsReport) {
	if agentID == "" || report == nil {
		return
	}

	s.mu.Lock()
	// Add to memory
	indices := s.byAgent[agentID]
	if len(indices) >= s.config.MaxMemoryEntries {
		oldIdx := indices[0]
		s.entries[oldIdx] = MetricsEntry{}
		s.byAgent[agentID] = append(indices[1:], s.head)
	} else {
		s.byAgent[agentID] = append(indices, s.head)
	}

	s.entries[s.head] = MetricsEntry{
		AgentID:  agentID,
		Report:   report,
		Received: time.Now(),
	}
	s.head = (s.head + 1) % s.config.MaxTotalEntries
	newSupEvents := s.ingestSupervisorEventsLocked(agentID, report.GetSupervisorEvents())
	onReport := s.onReport
	onSupEvent := s.onSupervisorEvent
	s.mu.Unlock()

	// Persist to database asynchronously
	go s.persistToDB(agentID, report)

	// 上报回调（异步，评估失败不影响存储）。
	if onReport != nil {
		go func() {
			defer func() {
				_ = recover() // 回调异常绝不拖垮上报链路
			}()
			evalCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			onReport(evalCtx, agentID, report)
		}()
	}

	// 新监管事件回调（异步逐事件，回调失败不影响存储与上报链路）。
	for _, ev := range newSupEvents {
		if onSupEvent == nil {
			break
		}
		go func(ev *opsv1.SupervisorEvent) {
			defer func() {
				_ = recover()
			}()
			evCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			onSupEvent(evCtx, agentID, ev)
		}(ev)
	}
}

// persistToDB saves metrics to database.
func (s *MetricsStore) persistToDB(agentID string, report *opsv1.MetricsReport) {
	db := s.db.Load()
	if db == nil {
		return
	}

	cpuJSON, _ := json.Marshal(report.GetCpu())
	memoryJSON, _ := json.Marshal(report.GetMemory())
	disksJSON, _ := json.Marshal(report.GetDisks())

	// 探测统计只有配探针面的 agent 才有；空切片不落列（NULL 而非 "[]"），
	// 读侧按列非空重建。
	var probeJSON dbtype.JSON
	if probes := report.GetProbeStats(); len(probes) > 0 {
		if raw, err := json.Marshal(probes); err == nil {
			probeJSON = dbtype.JSON(raw)
		}
	}
	// OPS-010 三面（网络速率/进程/端口）同规则：有数据才落列，老版本
	// agent 或关闭采集时保持 NULL，读侧按列非空重建。
	var networksJSON, processesJSON, portsJSON dbtype.JSON
	if nets := report.GetNetworks(); len(nets) > 0 {
		if raw, err := json.Marshal(nets); err == nil {
			networksJSON = dbtype.JSON(raw)
		}
	}
	if procs := report.GetProcesses(); len(procs) > 0 {
		if raw, err := json.Marshal(procs); err == nil {
			processesJSON = dbtype.JSON(raw)
		}
	}
	if ports := report.GetPorts(); len(ports) > 0 {
		if raw, err := json.Marshal(ports); err == nil {
			portsJSON = dbtype.JSON(raw)
		}
	}

	entry := AgentMetricsHistory{
		AgentID:       agentID,
		Timestamp:     time.Now(),
		CPUJSON:       cpuJSON,
		MemoryJSON:    memoryJSON,
		DisksJSON:     disksJSON,
		ProbeJSON:     probeJSON,
		NetworksJSON:  networksJSON,
		ProcessesJSON: processesJSON,
		PortsJSON:     portsJSON,
	}

	db.Create(&entry)
}

// ingestSupervisorEventsLocked merges piggybacked supervisor events into the
// per-agent ring and returns the newly ingested events (重传去重后) for the
// post-ingest callback. Caller must hold s.mu.
func (s *MetricsStore) ingestSupervisorEventsLocked(agentID string, evs []*opsv1.SupervisorEvent) []*opsv1.SupervisorEvent {
	if len(evs) == 0 {
		return nil
	}
	if s.supEvents == nil {
		s.supEvents = make(map[string][]*opsv1.SupervisorEvent)
		s.supLastSeq = make(map[string]int64)
		s.supLastTs = make(map[string]int64)
	}
	lastSeq, lastTs := s.supLastSeq[agentID], s.supLastTs[agentID]
	ring := s.supEvents[agentID]
	var fresh []*opsv1.SupervisorEvent
	for _, ev := range evs {
		if ev == nil {
			continue
		}
		// 重传去重：seq 与 ts 都不比已入库基线新才丢弃；seq 回退但 ts 更新
		// 说明 agent 重启后游标归零，按新事件重新入库。
		if ev.Seq <= lastSeq && ev.TsUnix <= lastTs {
			continue
		}
		ring = append(ring, ev)
		fresh = append(fresh, ev)
		if len(ring) > supervisorEventRingSize {
			ring = ring[len(ring)-supervisorEventRingSize:]
		}
		lastSeq, lastTs = ev.Seq, ev.TsUnix
	}
	s.supEvents[agentID] = ring
	s.supLastSeq[agentID] = lastSeq
	s.supLastTs[agentID] = lastTs
	return fresh
}

// GetSupervisorEvents returns buffered supervisor events for an agent, oldest
// first. sinceSeq > 0 keeps only events with a higher seq (incremental pull);
// limit > 0 caps the result to the newest events.
func (s *MetricsStore) GetSupervisorEvents(agentID string, sinceSeq int64, limit int) []*opsv1.SupervisorEvent {
	if agentID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*opsv1.SupervisorEvent
	for _, ev := range s.supEvents[agentID] {
		if ev == nil || (sinceSeq > 0 && ev.Seq <= sinceSeq) {
			continue
		}
		out = append(out, ev)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// SupervisorLatestSeq returns the newest buffered event seq for an agent
// (0 when none), so panels can bootstrap their incremental cursor.
func (s *MetricsStore) SupervisorLatestSeq(agentID string) int64 {
	if agentID == "" {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.supLastSeq[agentID]
}
