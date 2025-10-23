package loadbalancer

import (
	"context"
	"errors"
	"hash/fnv"
	"sort"

	"github.com/cuihairu/croupier/internal/server/registry"
)

// ConsistentHashBalancer implements consistent hashing load balancing
type ConsistentHashBalancer struct {
	healthCheck HealthChecker
	replicas    int // number of virtual nodes per agent
}

// NewConsistentHashBalancer creates a new consistent hash load balancer
func NewConsistentHashBalancer(replicas int, healthCheck HealthChecker) *ConsistentHashBalancer {
	if replicas <= 0 {
		replicas = 150 // default number of virtual nodes
	}
	return &ConsistentHashBalancer{
		replicas:    replicas,
		healthCheck: healthCheck,
	}
}

type hashRing struct {
	nodes map[uint32]string // hash -> agentID
	keys  []uint32          // sorted hash keys
}

func (c *ConsistentHashBalancer) Pick(ctx context.Context, candidates []*registry.AgentSession, key string) (*registry.AgentSession, error) {
	if len(candidates) == 0 {
		return nil, errors.New("no candidates available")
	}

	healthy := c.filterHealthy(candidates)
	if len(healthy) == 0 {
		return nil, errors.New("no healthy candidates available")
	}

	if key == "" {
		// No hash key provided, fallback to first healthy agent
		return healthy[0], nil
	}

	// Build hash ring
	ring := c.buildHashRing(healthy)
	if len(ring.keys) == 0 {
		return healthy[0], nil
	}

	// Hash the key
	hash := c.hash(key)

	// Find the first node >= hash
	idx := sort.Search(len(ring.keys), func(i int) bool {
		return ring.keys[i] >= hash
	})

	// Wrap around if necessary
	if idx == len(ring.keys) {
		idx = 0
	}

	targetAgentID := ring.nodes[ring.keys[idx]]

	// Find the agent session
	for _, agent := range healthy {
		if agent.AgentID == targetAgentID {
			return agent, nil
		}
	}

	// Fallback
	return healthy[0], nil
}

func (c *ConsistentHashBalancer) buildHashRing(agents []*registry.AgentSession) *hashRing {
	ring := &hashRing{
		nodes: make(map[uint32]string),
		keys:  make([]uint32, 0),
	}

	for _, agent := range agents {
		for i := 0; i < c.replicas; i++ {
			virtualKey := agent.AgentID + "#" + string(rune(i))
			hash := c.hash(virtualKey)
			ring.nodes[hash] = agent.AgentID
			ring.keys = append(ring.keys, hash)
		}
	}

	sort.Slice(ring.keys, func(i, j int) bool {
		return ring.keys[i] < ring.keys[j]
	})

	return ring
}

func (c *ConsistentHashBalancer) hash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

func (c *ConsistentHashBalancer) Name() string {
	return "consistent_hash"
}

func (c *ConsistentHashBalancer) UpdateHealth(agentID string, healthy bool) {
	if c.healthCheck != nil {
		c.healthCheck.SetHealthy(agentID, healthy)
	}
}

func (c *ConsistentHashBalancer) filterHealthy(candidates []*registry.AgentSession) []*registry.AgentSession {
	if c.healthCheck == nil {
		return candidates
	}

	var healthy []*registry.AgentSession
	for _, agent := range candidates {
		if c.healthCheck.IsHealthy(agent.AgentID) {
			healthy = append(healthy, agent)
		}
	}
	return healthy
}