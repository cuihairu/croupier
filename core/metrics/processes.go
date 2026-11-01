package metrics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// maxProcesses caps the per-report process list (top-N by RSS). The report
// rides the metrics channel; an unbounded list would blow up the payload on
// machines with thousands of processes. The node's listener list is not
// capped — it is bounded by actual listening sockets (typically < a few
// hundred).
const maxProcesses = 50

// listenerScan is one node-wide listener sweep shared by the process list
// (per-pid listening ports) and the port list (MetricsReport.ports). Doing
// the socket scan once per report avoids two identical kernel walks.
type listenerScan struct {
	// portsByPID maps pid → sorted, deduped listening ports.
	portsByPID map[int32][]int32
	// ports is the node-wide listener list (protocol, port, pid, name).
	ports []*opsv1.PortMetrics
}

// scanListeners walks tcp/tcp6/udp/udp6 sockets once and derives both faces.
// Protocol families the OS or process permissions cannot enumerate are
// silently skipped (the remaining families still report; the panel treats
// a missing family as "no data" rather than a hard failure).
func scanListeners(pidNames map[int32]string) listenerScan {
	scan := listenerScan{portsByPID: make(map[int32][]int32)}
	seen := make(map[string]bool)

	for _, kind := range []string{"tcp", "tcp6", "udp", "udp6"} {
		conns, err := net.Connections(kind)
		if err != nil {
			continue
		}
		for _, cn := range conns {
			// Listener predicates: TCP needs the LISTEN state; UDP has no
			// LISTEN state — a "listening" UDP socket is one bound to a local
			// port with no remote peer.
			isTCP := kind == "tcp" || kind == "tcp6"
			isUDP := kind == "udp" || kind == "udp6"
			if isTCP && !strings.EqualFold(cn.Status, "LISTEN") {
				continue
			}
			if isUDP && (cn.Laddr.Port == 0 || cn.Raddr.Port != 0) {
				continue
			}
			port := int32(cn.Laddr.Port)
			if port <= 0 {
				continue
			}
			key := kind + "/" + fmt.Sprintf("%d", port)
			if seen[key] {
				continue
			}
			seen[key] = true

			pid := cn.Pid
			name := pidNames[pid]
			scan.ports = append(scan.ports, &opsv1.PortMetrics{
				Port:        port,
				Protocol:    kind,
				Pid:         pid,
				ProcessName: name,
				State:       cn.Status,
			})
			scan.portsByPID[pid] = append(scan.portsByPID[pid], port)
		}
	}

	for pid := range scan.portsByPID {
		ports := scan.portsByPID[pid]
		sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })
	}
	return scan
}

// collectProcesses gathers the top-N processes by RSS plus the node-wide
// listener list. cpu_percent is the process's lifetime-average CPU usage
// (gopsutil CPUPercent semantics: total CPU time / time since start) — not an
// instantaneous rate, which would require per-process two-sample tracking.
func (c *Collector) collectProcesses() ([]*opsv1.ProcessMetrics, []*opsv1.PortMetrics) {
	procs, err := process.Processes()
	if err != nil || len(procs) == 0 {
		// No enumeration (restricted container / platform gap): process and
		// port faces are both empty rather than a fabricated list.
		return nil, nil
	}

	type snapshot struct {
		proc *process.Process
		name string
		mem  uint64
	}
	pidNames := make(map[int32]string, len(procs))
	snaps := make([]snapshot, 0, len(procs))
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			name = fmt.Sprintf("%d", p.Pid)
		}
		pidNames[p.Pid] = name
		var mem uint64
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			mem = mi.RSS
		}
		snaps = append(snaps, snapshot{proc: p, name: name, mem: mem})
	}

	sort.Slice(snaps, func(i, j int) bool { return snaps[i].mem > snaps[j].mem })
	if len(snaps) > maxProcesses {
		snaps = snaps[:maxProcesses]
	}

	scan := scanListeners(pidNames)

	out := make([]*opsv1.ProcessMetrics, 0, len(snaps))
	for _, s := range snaps {
		pm := &opsv1.ProcessMetrics{
			Pid:         s.proc.Pid,
			Name:        s.name,
			MemoryBytes: s.mem,
		}
		if st, err := s.proc.Status(); err == nil && len(st) > 0 {
			pm.Status = st[0]
		}
		if n, err := s.proc.NumThreads(); err == nil {
			pm.Threads = int32(n)
		}
		if created, err := s.proc.CreateTime(); err == nil {
			pm.StartTime = timestamppb.New(time.UnixMilli(created))
		}
		if cpu, err := s.proc.CPUPercent(); err == nil {
			pm.CpuPercent = cpu
		}
		if ports := scan.portsByPID[s.proc.Pid]; len(ports) > 0 {
			pm.ListeningPorts = ports
		}
		out = append(out, pm)
	}

	return out, scan.ports
}
