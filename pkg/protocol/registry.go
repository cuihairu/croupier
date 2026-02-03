// Package protocol provides message type registry for Croupier protocol.
package protocol

import (
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
)

// Registry maps message IDs to protobuf message factories.
type Registry struct {
	mu      sync.RWMutex
	factory map[uint32]func() proto.Message
}

// NewRegistry creates a new empty message registry.
func NewRegistry() *Registry {
	return &Registry{
		factory: make(map[uint32]func() proto.Message),
	}
}

// Register registers a message type with its factory function.
// The factory should return a new instance of the protobuf message.
func (r *Registry) Register(msgID uint32, factory func() proto.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factory[msgID] = factory
}

// RegisterBatch registers multiple message types at once.
func (r *Registry) RegisterBatch(types map[uint32]func() proto.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for msgID, factory := range types {
		r.factory[msgID] = factory
	}
}

// Create creates a new instance of the message with the given MsgID.
func (r *Registry) Create(msgID uint32) (proto.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.factory[msgID]
	if !ok {
		return nil, fmt.Errorf("unknown message type: 0x%06X", msgID)
	}
	return factory(), nil
}

// Unmarshal unmarshals the body bytes into the appropriate message type.
func (r *Registry) Unmarshal(msgID uint32, body []byte) (proto.Message, error) {
	msg, err := r.Create(msgID)
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(body, msg); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", MsgIDString(msgID), err)
	}
	return msg, nil
}

// MustRegister registers a message type and panics on error.
// Useful for package-level initialization.
func (r *Registry) MustRegister(msgID uint32, factory func() proto.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factory[msgID]; exists {
		panic(fmt.Sprintf("message type 0x%06X already registered", msgID))
	}
	r.factory[msgID] = factory
}
