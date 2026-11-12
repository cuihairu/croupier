package metrics

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
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

// maxWalkWorkers bounds the parallel per-pid reads. Process-table walks are
// many independent small procfs reads; a worker pool keeps the wall clock at
// a few reads' latency even on hosts with tens of thousands of processes
// (leaked/zombie storms make the table huge, and the collector is supposed
// to keep observing precisely there).
const maxWalkWorkers = 8

// listenerScan is one node-wide listener sweep shared by the process list
// (per-pid listening ports) and the port list (MetricsReport.ports). Doing
// the socket scan once per report avoids two identical kernel walks.
type listenerScan struct {
	// portsByPID maps pid → sorted, deduped listening ports.
	portsByPID map[int32][]int32
	// ports is the node-wide listener list (protocol, port, pid, name).
	ports []*opsv1.PortMetrics
}

// listenerConnections fetches all inet sockets in one gopsutil call. On
// Linux "inet" (tcp4+tcp6+udp4+udp6) shares a single /proc/<pid>/fd walk
// inside gopsutil; asking per protocol repeats that whole walk four times,
// which costs seconds per report on hosts with huge process tables.
// Non-Linux platforms may not know the "inet" kind — fall back to the
// per-protocol calls (the historical behavior, absorbed per-call errors).
func listenerConnections() []net.ConnectionStat {
	conns, err := net.Connections("inet")
	if err == nil && len(conns) > 0 {
		return conns
	}
	var out []net.ConnectionStat
	for _, kind := range []string{"tcp", "tcp6", "udp", "udp6"} {
		cs, err := net.Connections(kind)
		if err != nil {
			continue
		}
		out = append(out, cs...)
	}
	return out
}

// listenerKind maps a socket's (family, type) back to the protocol label the
// port face reports ("tcp"/"tcp6"/"udp"/"udp6"); non-inet sockets (unix) are
// dropped.
func listenerKind(cn net.ConnectionStat) (string, bool) {
	switch {
	case cn.Type == syscall.SOCK_STREAM && cn.Family == syscall.AF_INET:
		return "tcp", true
	case cn.Type == syscall.SOCK_STREAM && cn.Family == syscall.AF_INET6:
		return "tcp6", true
	case cn.Type == syscall.SOCK_DGRAM && cn.Family == syscall.AF_INET:
		return "udp", true
	case cn.Type == syscall.SOCK_DGRAM && cn.Family == syscall.AF_INET6:
		return "udp6", true
	}
	return "", false
}

// scanListeners walks the inet socket table once and derives both faces.
// Protocol families the OS or process permissions cannot enumerate are
// silently skipped (the remaining families still report; the panel treats
// a missing family as "no data" rather than a hard failure).
func scanListeners(pidNames map[int32]string) listenerScan {
	scan := listenerScan{portsByPID: make(map[int32][]int32)}
	seen := make(map[string]bool)

	for _, cn := range listenerConnections() {
		kind, ok := listenerKind(cn)
		if !ok {
			continue
		}
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
	pids, err := process.Pids()
	if err != nil || len(pids) == 0 {
		// No enumeration (restricted container / platform gap): process and
		// port faces are both empty rather than a fabricated list.
		return nil, nil
	}

	type snapshot struct {
		proc *process.Process
		name string
		mem  uint64
	}

	// Direct &process.Process{Pid: pid} construction instead of
	// process.Processes(): the latter runs PidExists + a CreateTime
	// /proc/<pid>/stat read per pid and throws both away — pure overhead at
	// tens of thousands of pids. Existence is implicit: an exited pid's
	// Name()/MemoryInfo() errors land in the existing fallbacks.
	workers := runtime.NumCPU()
	if workers > maxWalkWorkers {
		workers = maxWalkWorkers
	}
	if workers < 2 {
		workers = 2
	}
	pidCh := make(chan int32)
	snapCh := make(chan snapshot, 64)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pid := range pidCh {
				p := &process.Process{Pid: pid}
				name, err := p.Name()
				if err != nil || name == "" {
					name = fmt.Sprintf("%d", pid)
				}
				var mem uint64
				if mi, err := p.MemoryInfo(); err == nil && mi != nil {
					mem = mi.RSS
				}
				snapCh <- snapshot{proc: p, name: name, mem: mem}
			}
		}()
	}
	go func() {
		for _, pid := range pids {
			pidCh <- pid
		}
		close(pidCh)
	}()
	go func() {
		wg.Wait()
		close(snapCh)
	}()

	pidNames := make(map[int32]string, len(pids))
	snaps := make([]snapshot, 0, len(pids))
	for s := range snapCh {
		pidNames[s.proc.Pid] = s.name
		snaps = append(snaps, s)
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
