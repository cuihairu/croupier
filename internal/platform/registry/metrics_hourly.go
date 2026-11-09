package registry

import (
	"context"
	"encoding/json"

	"time"

	"github.com/cuihairu/croupier/internal/common/dbtype"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// RollupHourly aggregates the previous completed hour's raw rows into
// agent_metrics_hourly (OPS-010 聚合 90 天）。Idempotent per (agent, hour):
// re-rolling the same hour overwrites the row. Raw rows older than
// MetricsRetention are left to Prune.
func (s *MetricsStore) RollupHourly(ctx context.Context) {
	db := s.db.Load()
	if db == nil {
		return
	}
	now := time.Now()
	// Roll up the hour that just completed: [now-1h truncated to hour, +1h).
	hourStart := now.Add(-time.Hour).Truncate(time.Hour)
	hourEnd := hourStart.Add(time.Hour)

	var rows []AgentMetricsHistory
	if err := db.WithContext(ctx).
		Where("timestamp >= ? AND timestamp < ?", hourStart, hourEnd).
		Find(&rows).Error; err != nil {
		return
	}

	byAgent := make(map[string][]AgentMetricsHistory)
	for _, r := range rows {
		if r.AgentID == "" {
			continue
		}
		byAgent[r.AgentID] = append(byAgent[r.AgentID], r)
	}

	for agentID, agentRows := range byAgent {
		if len(agentRows) == 0 {
			continue
		}
		agg := aggregateHourly(agentRows)
		if agg.SampleCount == 0 {
			continue
		}
		agg.AgentID = agentID
		agg.HourStart = hourStart
		// Upsert: one row per (agent, hour). The compound unique index
		// (uidx_agent_hour) is the invariant; find-or-create stays portable
		// across sqlite/pg/mysql (no dialect-specific ON CONFLICT). When two
		// servers (or a cleanup tick and a manual trigger) race, the losing
		// Create hits the unique constraint and retries once: re-read the
		// winner and Save onto it — no duplicate row, no lost aggregate.
		var existing AgentMetricsHourly
		err := db.WithContext(ctx).
			Where("agent_id = ? AND hour_start = ?", agentID, hourStart).
			First(&existing).Error
		if err != nil {
			// No row yet — create.
			if e := db.WithContext(ctx).Create(&agg).Error; e != nil {
				// 竞态：他协程/实例已插入同一 (agent_id, hour_start)，唯一
				// 约束拒绝本次 Create。重读胜者行并更新，保证不重复且不丢
				// 聚合。
				var winner AgentMetricsHourly
				if rerr := db.WithContext(ctx).
					Where("agent_id = ? AND hour_start = ?", agentID, hourStart).
					First(&winner).Error; rerr != nil {
					continue
				}
				agg.ID = winner.ID
				db.WithContext(ctx).Save(&agg)
				continue
			}
			continue
		}
		agg.ID = existing.ID
		db.WithContext(ctx).Save(&agg)
	}
}

// aggregateHourly averages one agent's raw rows into a single hourly row.
func aggregateHourly(rows []AgentMetricsHistory) AgentMetricsHourly {
	var cpuSum, memSum, procSum, portSum float64
	var cpuN, memN, procN, portN int
	diskByMount := make(map[string]*diskAccum)
	netByIface := make(map[string]*netRateAccum)

	for _, r := range rows {
		if len(r.CPUJSON) > 0 {
			var cpu opsv1.CpuMetrics
			if json.Unmarshal(r.CPUJSON, &cpu) == nil {
				cpuSum += cpu.GetUsagePercent()
				cpuN++
			}
		}
		if len(r.MemoryJSON) > 0 {
			var mem opsv1.MemoryMetrics
			if json.Unmarshal(r.MemoryJSON, &mem) == nil {
				memSum += mem.GetUsagePercent()
				memN++
			}
		}
		if len(r.DisksJSON) > 0 {
			var disks []*opsv1.DiskMetrics
			if json.Unmarshal(r.DisksJSON, &disks) == nil {
				for _, d := range disks {
					acc := diskByMount[d.GetMountPoint()]
					if acc == nil {
						acc = &diskAccum{}
						diskByMount[d.GetMountPoint()] = acc
					}
					acc.usage += d.GetUsagePercent()
					acc.n++
				}
			}
		}
		if len(r.NetworksJSON) > 0 {
			var nets []*opsv1.NetworkMetrics
			if json.Unmarshal(r.NetworksJSON, &nets) == nil {
				for _, n := range nets {
					acc := netByIface[n.GetInterface()]
					if acc == nil {
						acc = &netRateAccum{}
						netByIface[n.GetInterface()] = acc
					}
					acc.sent += n.GetBytesSentRate()
					acc.recv += n.GetBytesRecvRate()
					acc.n++
				}
			}
		}
		if len(r.ProcessesJSON) > 0 {
			var procs []*opsv1.ProcessMetrics
			if json.Unmarshal(r.ProcessesJSON, &procs) == nil {
				procSum += float64(len(procs))
				procN++
			}
		}
		if len(r.PortsJSON) > 0 {
			var ports []*opsv1.PortMetrics
			if json.Unmarshal(r.PortsJSON, &ports) == nil {
				portSum += float64(len(ports))
				portN++
			}
		}
	}

	agg := AgentMetricsHourly{SampleCount: len(rows)}
	if cpuN > 0 {
		agg.CPUUsageAvg = cpuSum / float64(cpuN)
	}
	if memN > 0 {
		agg.MemUsageAvg = memSum / float64(memN)
	}
	if procN > 0 {
		agg.ProcessCountAvg = procSum / float64(procN)
	}
	if portN > 0 {
		agg.PortCountAvg = portSum / float64(portN)
	}

	if len(diskByMount) > 0 {
		type diskAvg struct {
			MountPoint   string  `json:"mount_point"`
			UsagePercent float64 `json:"usage_percent"`
		}
		out := make([]diskAvg, 0, len(diskByMount))
		for mount, acc := range diskByMount {
			avg := 0.0
			if acc.n > 0 {
				avg = acc.usage / float64(acc.n)
			}
			out = append(out, diskAvg{MountPoint: mount, UsagePercent: round2(avg)})
		}
		if raw, err := json.Marshal(out); err == nil {
			agg.DisksJSON = dbtype.JSON(raw)
		}
	}
	if len(netByIface) > 0 {
		type netAvg struct {
			Interface     string  `json:"interface"`
			BytesSentRate float64 `json:"bytes_sent_rate"`
			BytesRecvRate float64 `json:"bytes_recv_rate"`
		}
		out := make([]netAvg, 0, len(netByIface))
		for iface, acc := range netByIface {
			out = append(out, netAvg{
				Interface:     iface,
				BytesSentRate: round2(acc.sent / float64(acc.n)),
				BytesRecvRate: round2(acc.recv / float64(acc.n)),
			})
		}
		if raw, err := json.Marshal(out); err == nil {
			agg.NetRatesJSON = dbtype.JSON(raw)
		}
	}

	return agg
}

// netRateAccum accumulates per-interface rate samples within an hour.
type netRateAccum struct {
	sent float64
	recv float64
	n    int
}

type diskAccum struct {
	usage float64
	n     int
}

// round2 rounds to two decimals (percent/rate display precision).
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// GetHourlyHistory returns hourly aggregate history for an agent, oldest
// first, newest ≤ limit. Rows are rebuilt into synthetic MetricsEntry
// reports (CPU/Memory averages + a Disks/Networks projection) so the panel
// can render 90-day curves through the same entry shape.
func (s *MetricsStore) GetHourlyHistory(agentID string, since time.Time, limit int) []MetricsEntry {
	db := s.db.Load()
	if agentID == "" || db == nil {
		return nil
	}
	var rows []AgentMetricsHourly
	q := db.Where("agent_id = ?", agentID)
	if !since.IsZero() {
		q = q.Where("hour_start >= ?", since)
	}
	q = q.Order("hour_start DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	q.Find(&rows)

	// Reverse to oldest-first to match GetHistory ordering.
	result := make([]MetricsEntry, len(rows))
	for i := range rows {
		result[i] = metricsEntryFromHourly(rows[len(rows)-1-i])
	}
	return result
}

func metricsEntryFromHourly(row AgentMetricsHourly) MetricsEntry {
	report := &opsv1.MetricsReport{
		AgentId: row.AgentID,
		Cpu:     &opsv1.CpuMetrics{UsagePercent: row.CPUUsageAvg},
		Memory:  &opsv1.MemoryMetrics{UsagePercent: row.MemUsageAvg},
	}
	if len(row.DisksJSON) > 0 {
		var disks []struct {
			MountPoint   string  `json:"mount_point"`
			UsagePercent float64 `json:"usage_percent"`
		}
		if json.Unmarshal(row.DisksJSON, &disks) == nil {
			for _, d := range disks {
				report.Disks = append(report.Disks, &opsv1.DiskMetrics{
					MountPoint:   d.MountPoint,
					UsagePercent: d.UsagePercent,
				})
			}
		}
	}
	if len(row.NetRatesJSON) > 0 {
		var nets []struct {
			Interface     string  `json:"interface"`
			BytesSentRate float64 `json:"bytes_sent_rate"`
			BytesRecvRate float64 `json:"bytes_recv_rate"`
		}
		if json.Unmarshal(row.NetRatesJSON, &nets) == nil {
			for _, n := range nets {
				report.Networks = append(report.Networks, &opsv1.NetworkMetrics{
					Interface:     n.Interface,
					BytesSentRate: n.BytesSentRate,
					BytesRecvRate: n.BytesRecvRate,
				})
			}
		}
	}
	return MetricsEntry{
		AgentID:  row.AgentID,
		Report:   report,
		Received: row.HourStart,
	}
}
