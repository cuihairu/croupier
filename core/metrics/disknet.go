package metrics

import (
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/net"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func (c *Collector) collectDisks() []*opsv1.DiskMetrics {
	var result []*opsv1.DiskMetrics

	partitions, err := disk.Partitions(false) // false = only physical disks
	if err != nil {
		return result
	}

	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		result = append(result, &opsv1.DiskMetrics{
			MountPoint:     p.Mountpoint,
			Device:         p.Device,
			FsType:         p.Fstype,
			TotalBytes:     usage.Total,
			UsedBytes:      usage.Used,
			AvailableBytes: usage.Free,
			UsagePercent:   usage.UsedPercent,
			InodeTotal:     usage.InodesTotal,
			InodeUsed:      usage.InodesUsed,
		})
	}

	return result
}

// collectNetworks gathers per-interface counters and derives per-report-window
// rates (bytes/packets per second) from the delta against the previous scan
// (OPS-010 网络收发速率）。The cumulative counters stay in the report for
// the total-since-boot face; the rate fields are what the panel curves plot.
// The first scan after agent start has no prior sample → rate 0.
func (c *Collector) collectNetworks() []*opsv1.NetworkMetrics {
	var result []*opsv1.NetworkMetrics

	counters, err := net.IOCounters(true) // true = per interface
	if err != nil {
		return result
	}

	now := time.Now()
	c.netPrevMu.Lock()
	defer c.netPrevMu.Unlock()
	if c.netPrev == nil {
		c.netPrev = make(map[string]netCounterSample)
	}

	for _, counter := range counters {
		// Skip loopback interface
		if counter.Name == "lo" || counter.Name == "lo0" {
			continue
		}

		m := &opsv1.NetworkMetrics{
			Interface:   counter.Name,
			BytesSent:   counter.BytesSent,
			BytesRecv:   counter.BytesRecv,
			PacketsSent: counter.PacketsSent,
			PacketsRecv: counter.PacketsRecv,
			ErrorsIn:    counter.Errin,
			ErrorsOut:   counter.Errout,
		}

		if prev, ok := c.netPrev[counter.Name]; ok {
			dt := now.Sub(prev.ts).Seconds()
			if dt > 0 {
				m.BytesSentRate = counterRate(counter.BytesSent, prev.bytesSent, dt)
				m.BytesRecvRate = counterRate(counter.BytesRecv, prev.bytesRecv, dt)
				m.PacketsSentRate = counterRate(counter.PacketsSent, prev.packetsSent, dt)
				m.PacketsRecvRate = counterRate(counter.PacketsRecv, prev.packetsRecv, dt)
			}
		}
		c.netPrev[counter.Name] = netCounterSample{
			bytesSent:   counter.BytesSent,
			bytesRecv:   counter.BytesRecv,
			packetsSent: counter.PacketsSent,
			packetsRecv: counter.PacketsRecv,
			ts:          now,
		}

		result = append(result, m)
	}

	return result
}

// netCounterSample is the previous scan's cumulative counters for one
// interface, kept so the next scan can compute a rate delta.
type netCounterSample struct {
	bytesSent   uint64
	bytesRecv   uint64
	packetsSent uint64
	packetsRecv uint64
	ts          time.Time
}

// counterRate is a delta-per-second rate. A counter that went backwards
// (interface recreated, agent reboot) yields 0 rather than a negative rate.
func counterRate(cur, prev uint64, dt float64) float64 {
	if dt <= 0 || cur < prev {
		return 0
	}
	return float64(cur-prev) / dt
}
