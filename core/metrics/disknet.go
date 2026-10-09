package metrics

import (
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

func (c *Collector) collectNetworks() []*opsv1.NetworkMetrics {
	var result []*opsv1.NetworkMetrics

	counters, err := net.IOCounters(true) // true = per interface
	if err != nil {
		return result
	}

	for _, counter := range counters {
		// Skip loopback interface
		if counter.Name == "lo" || counter.Name == "lo0" {
			continue
		}

		result = append(result, &opsv1.NetworkMetrics{
			Interface:   counter.Name,
			BytesSent:   counter.BytesSent,
			BytesRecv:   counter.BytesRecv,
			PacketsSent: counter.PacketsSent,
			PacketsRecv: counter.PacketsRecv,
			ErrorsIn:    counter.Errin,
			ErrorsOut:   counter.Errout,
		})
	}

	return result
}
