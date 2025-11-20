package svc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cuihairu/croupier/services/edge/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config       config.Config
	TunnelMgr    *TunnelManager
	ProxyMgr     *ProxyManager
	LoadBalancer *LoadBalancer
}

type Tunnel struct {
	ID           string                 `json:"id"`
	AgentID      string                 `json:"agent_id"`
	ServerID     string                 `json:"server_id"`
	Protocol     string                 `json:"protocol"`
	RemoteAddr   string                 `json:"remote_addr"`
	LocalAddr    string                 `json:"local_addr"`
	Options      map[string]interface{} `json:"options"`
	Status       string                 `json:"status"`
	Connections  int64                  `json:"connections"`
	BytesIn      int64                  `json:"bytes_in"`
	BytesOut     int64                  `json:"bytes_out"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActive   time.Time              `json:"last_active"`
	PublicURL    string                 `json:"public_url"`
	Listener     net.Listener           `json:"-"`
	ctx          context.Context
	cancelFunc   context.CancelFunc
}

type ProxyConnection struct {
	ID        string    `json:"id"`
	TunnelID  string    `json:"tunnel_id"`
	ClientConn net.Conn `json:"-"`
	ServerConn net.Conn `json:"-"`
	StartTime time.Time `json:"start_time"`
	BytesIn   int64     `json:"bytes_in"`
	BytesOut  int64     `json:"bytes_out"`
	Active    bool      `json:"active"`
}

type TunnelManager struct {
	mu       sync.RWMutex
	tunnels  map[string]*Tunnel
	maxCount int
}

type ProxyManager struct {
	mu          sync.RWMutex
	connections map[string]*ProxyConnection
	maxConn     int
}

type LoadBalancer struct {
	strategy    string
	healthCheck bool
	tunnels     []*Tunnel
	current     int
	mu          sync.Mutex
}

func NewServiceContext(c config.Config) *ServiceContext {
	logx.Info("Initializing edge service context")

	return &ServiceContext{
		Config:       c,
		TunnelMgr:    NewTunnelManager(c.Tunnel.MaxTunnels),
		ProxyMgr:     NewProxyManager(c.Proxy.MaxConnections),
		LoadBalancer: NewLoadBalancer(c.LoadBalancer.Strategy),
	}
}

func NewTunnelManager(maxTunnels int) *TunnelManager {
	return &TunnelManager{
		tunnels:  make(map[string]*Tunnel),
		maxCount: maxTunnels,
	}
}

func NewProxyManager(maxConnections int) *ProxyManager {
	return &ProxyManager{
		connections: make(map[string]*ProxyConnection),
		maxConn:     maxConnections,
	}
}

func NewLoadBalancer(strategy string) *LoadBalancer {
	return &LoadBalancer{
		strategy: strategy,
		tunnels:  make([]*Tunnel, 0),
		current:  0,
	}
}

func (tm *TunnelManager) CreateTunnel(tunnel *Tunnel) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if len(tm.tunnels) >= tm.maxCount {
		return false
	}

	if _, exists := tm.tunnels[tunnel.ID]; exists {
		return false
	}

	tunnel.ctx, tunnel.cancelFunc = context.WithCancel(context.Background())
	tunnel.CreatedAt = time.Now()
	tunnel.LastActive = time.Now()
	tunnel.Status = "active"

	tm.tunnels[tunnel.ID] = tunnel
	return true
}

func (tm *TunnelManager) GetTunnel(id string) (*Tunnel, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	tunnel, ok := tm.tunnels[id]
	return tunnel, ok
}

func (tm *TunnelManager) UpdateTunnel(id string, updateFunc func(*Tunnel)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tunnel, ok := tm.tunnels[id]; ok {
		updateFunc(tunnel)
		tunnel.LastActive = time.Now()
	}
}

func (tm *TunnelManager) CloseTunnel(id string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tunnel, ok := tm.tunnels[id]
	if !ok {
		return false
	}

	tunnel.Status = "closed"
	if tunnel.cancelFunc != nil {
		tunnel.cancelFunc()
	}
	if tunnel.Listener != nil {
		tunnel.Listener.Close()
	}

	delete(tm.tunnels, id)
	return true
}

func (tm *TunnelManager) ListTunnels(agentID, status string, page, size int) ([]*Tunnel, int64) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var filtered []*Tunnel
	for _, tunnel := range tm.tunnels {
		if agentID != "" && tunnel.AgentID != agentID {
			continue
		}
		if status != "" && tunnel.Status != status {
			continue
		}
		filtered = append(filtered, tunnel)
	}

	total := int64(len(filtered))
	if page <= 0 || size <= 0 {
		return filtered, total
	}

	start := (page - 1) * size
	end := start + size
	if start >= len(filtered) {
		return []*Tunnel{}, total
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], total
}

func (pm *ProxyManager) CreateConnection(conn *ProxyConnection) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if len(pm.connections) >= pm.maxConn {
		return false
	}

	pm.connections[conn.ID] = conn
	return true
}

func (pm *ProxyManager) GetConnection(id string) (*ProxyConnection, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	conn, ok := pm.connections[id]
	return conn, ok
}

func (pm *ProxyManager) CloseConnection(id string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	conn, ok := pm.connections[id]
	if !ok {
		return false
	}

	conn.Active = false
	if conn.ClientConn != nil {
		conn.ClientConn.Close()
	}
	if conn.ServerConn != nil {
		conn.ServerConn.Close()
	}

	delete(pm.connections, id)
	return true
}

func (pm *ProxyManager) UpdateConnection(id string, bytesIn, bytesOut int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if conn, ok := pm.connections[id]; ok {
		conn.BytesIn += bytesIn
		conn.BytesOut += bytesOut
	}
}

func (lb *LoadBalancer) AddTunnel(tunnel *Tunnel) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.tunnels = append(lb.tunnels, tunnel)
}

func (lb *LoadBalancer) RemoveTunnel(tunnelID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for i, t := range lb.tunnels {
		if t.ID == tunnelID {
			lb.tunnels = append(lb.tunnels[:i], lb.tunnels[i+1:]...)
			break
		}
	}
}

func (lb *LoadBalancer) SelectTunnel() (*Tunnel, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.tunnels) == 0 {
		return nil, fmt.Errorf("no available tunnels")
	}

	// Simple round-robin for now
	// TODO: Implement other strategies (least_conn, ip_hash)
	switch lb.strategy {
	case "round_robin":
		fallthrough
	default:
		tunnel := lb.tunnels[lb.current%len(lb.tunnels)]
		lb.current++
		if tunnel.Status != "active" {
			return lb.SelectTunnel() // Try next tunnel
		}
		return tunnel, nil
	}
}