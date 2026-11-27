// Package protocol implements the Croupier wire protocol.
//
// Message Format:
//
//	Header (8 bytes):
//	  ┌─────────┬──────────┬─────────────────┐
//	  │ Version │ MsgID    │ RequestID       │
//	  │ (1B)    │ (3B)     │ (4B)            │
//	  └─────────┴──────────┴─────────────────┘
//
//	Body: protobuf serialized message
//
// Request messages have odd MsgID, Response messages have even MsgID.
// The RequestID is used to match responses to their requests.
package protocol

import (
	"encoding/binary"
	"fmt"
	"sync"
)

const (
	// Version1 is the current protocol version.
	Version1 = 0x01

	// MinSupportedVersion is the oldest wire protocol version this SDK
	// accepts. Keep aligned with the server's version window (N-1 policy).
	MinSupportedVersion = Version1

	// HeaderSize is the fixed size of the message header in bytes.
	HeaderSize = 8 // Version(1) + MsgID(3) + RequestID(4)
)

// SupportedVersions returns the sorted list of wire protocol versions this
// SDK accepts.
//
// The list is derived from the accepted window, so when the N-1 floor is
// opened (MinSupportedVersion < Version1) rejection payloads report the old
// versions too, instead of under-reporting as current-only.
func SupportedVersions() []int {
	versions := make([]int, 0, int(Version1)-int(MinSupportedVersion)+1)
	for v := int(Version1); v >= int(MinSupportedVersion); v-- {
		versions = append(versions, v)
	}
	return versions
}

// IsSupportedVersion reports whether a wire protocol version is within the
// accepted window [MinSupportedVersion, Version1].
func IsSupportedVersion(version uint8) bool {
	return version >= MinSupportedVersion && version <= Version1
}

// ErrVersionConflict reports an in-window frame version that deviates from
// the version already locked for the connection (AUD-101 sticky negotiation).
type ErrVersionConflict struct{ Locked, Got uint8 }

func (e *ErrVersionConflict) Error() string {
	return fmt.Sprintf("frame version %d deviates from negotiated version %d", e.Got, e.Locked)
}

// Negotiation owns the per-connection negotiated frame version (AUD-101).
// The header version doubles as the negotiation carrier — v1 defines no
// explicit handshake. The initiator stamps its maximum supported version on
// the first frame; the receiver locks the connection to the first supported
// inbound version, and every later frame on that connection (requests,
// responses, proactive events) carries the negotiated version. A fresh TCP
// connection negotiates from scratch. Mirrors pkg/protocol in the main
// repository so SDK and server converge on the same rule.
type Negotiation struct {
	mu      sync.Mutex
	locked  bool
	version uint8
	min     uint8
	max     uint8
}

// NewNegotiation builds a Negotiation over the package default accepted
// window [MinSupportedVersion, Version1].
func NewNegotiation() *Negotiation {
	return NewNegotiationWindow(MinSupportedVersion, Version1)
}

// NewNegotiationWindow builds a Negotiation over an explicit accepted
// window so tests can rehearse the next version bump (N/N-1 mixes) before
// Version1 moves.
func NewNegotiationWindow(min, max uint8) *Negotiation {
	if min == 0 || max < min {
		panic(fmt.Sprintf("protocol: invalid version window [%d,%d]", min, max))
	}
	return &Negotiation{version: max, min: min, max: max}
}

// Offer records the peer's frame version. When unlocked it locks the
// negotiated version to peer (peer must be inside the accepted window) and
// returns it; when already locked it returns the negotiated version, or an
// ErrVersionConflict when the peer deviates from the locked version.
func (n *Negotiation) Offer(peer uint8) (uint8, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.locked {
		if peer != n.version {
			return n.version, &ErrVersionConflict{Locked: n.version, Got: peer}
		}
		return n.version, nil
	}
	if peer < n.min || peer > n.max {
		return n.version, fmt.Errorf("version %d outside supported window [%d,%d]", peer, n.min, n.max)
	}
	n.locked = true
	n.version = peer
	return n.version, nil
}

// Version returns the frame version outbound writes must use: the negotiated
// version once locked, otherwise the local maximum (the initiator's natural
// offer). Thread-safe.
func (n *Negotiation) Version() uint8 {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.version
}

// Accepts reports whether a frame version is inside this connection's
// accepted window. With the default window it mirrors IsSupportedVersion;
// an injected window (tests rehearsing the next bump) widens it per
// connection only.
func (n *Negotiation) Accepts(version uint8) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return version >= n.min && version <= n.max
}

// Message type constants (24 bits).
const (
	// ControlService (0x01xx)
	MsgRegisterRequest          = 0x010101
	MsgRegisterResponse         = 0x010102
	MsgHeartbeatRequest         = 0x010103
	MsgHeartbeatResponse        = 0x010104
	MsgRegisterCapabilitiesReq  = 0x010105
	MsgRegisterCapabilitiesResp = 0x010106

	// ClientService (0x02xx)
	MsgRegisterClientRequest   = 0x020101
	MsgRegisterClientResponse  = 0x020102
	MsgClientHeartbeatRequest  = 0x020103
	MsgClientHeartbeatResponse = 0x020104
	MsgListClientsRequest      = 0x020105
	MsgListClientsResponse     = 0x020106

	// InvokerService (0x03xx)
	MsgInvokeRequest      = 0x030101
	MsgInvokeResponse     = 0x030102
	MsgStartTaskRequest   = 0x030103
	MsgStartTaskResponse  = 0x030104
	MsgStreamTaskRequest  = 0x030105
	MsgTaskEvent          = 0x030106 // Stream event (not request/response)
	MsgCancelTaskRequest  = 0x030107
	MsgCancelTaskResponse = 0x030108

	// ProviderService (0x05xx) - Provider session protocol
	MsgProviderConnectRequest    = 0x050101
	MsgProviderConnectResponse   = 0x050102
	MsgProviderHeartbeatRequest  = 0x050103
	MsgProviderHeartbeatResponse = 0x050104
	MsgProviderDrainRequest      = 0x050105
	MsgProviderDrainResponse     = 0x050106
	MsgGetTaskResultRequest      = 0x050107
	MsgGetTaskResultResponse     = 0x050108

	// Provider file push (F：文件下发原语，hotpatch P1 传输层)
	MsgProviderFilePushRequest  = 0x050109
	MsgProviderFilePushResponse = 0x05010A
)

// PutMsgID encodes a 24-bit MsgID into buf in big-endian order.
func PutMsgID(buf []byte, msgID uint32) {
	buf[0] = byte(msgID >> 16)
	buf[1] = byte(msgID >> 8)
	buf[2] = byte(msgID)
}

// GetMsgID decodes a 24-bit MsgID from buf in big-endian order.
func GetMsgID(buf []byte) uint32 {
	return uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])
}

// NewMessageBody creates a body with protocol header prefix and data.
// Body format: [8-byte header][actual data]
func NewMessageBody(msgID uint32, reqID uint32, data []byte) []byte {
	return NewMessageBodyWithVersion(Version1, msgID, reqID, data)
}

// NewMessageBodyWithVersion creates a body stamped with an explicit frame
// version — the per-connection negotiated version (AUD-101). Version 0 is
// not a valid wire version and is treated as Version1 to keep the
// historical single-version callers safe.
func NewMessageBodyWithVersion(version uint8, msgID uint32, reqID uint32, data []byte) []byte {
	body := make([]byte, HeaderSize+len(data))
	if version == 0 {
		version = Version1
	}
	body[0] = version
	PutMsgID(body[1:4], msgID)
	binary.BigEndian.PutUint32(body[4:8], reqID)
	copy(body[HeaderSize:], data)
	return body
}

// ParseMessageFromBody parses a message from a body that contains the protocol header as a prefix.
// Body format: [8-byte header][actual data]
func ParseMessageFromBody(body []byte) (version uint8, msgID uint32, reqID uint32, data []byte, err error) {
	if len(body) < HeaderSize {
		err = fmt.Errorf("body too short for header: %d < %d", len(body), HeaderSize)
		return
	}

	version = body[0]
	msgID = GetMsgID(body[1:4])
	reqID = binary.BigEndian.Uint32(body[4:8])
	data = body[HeaderSize:]

	return
}

// IsRequest returns true if the MsgID indicates a request message.
func IsRequest(msgID uint32) bool {
	return msgID%2 == 1 && msgID != MsgTaskEvent
}

// controlRequests is the explicit set of session/transport control requests
// (dual-lane dispatch: control traffic never competes with the bounded
// business queue — heartbeats and drain stay reachable under overload).
var controlRequests = map[uint32]struct{}{
	// Agent session control (0x01xx)
	MsgRegisterRequest:         {},
	MsgHeartbeatRequest:        {},
	MsgRegisterCapabilitiesReq: {},
	// Client session control (0x02xx)
	MsgRegisterClientRequest:  {},
	MsgClientHeartbeatRequest: {},
	// Provider session control (0x05xx), incl. drain
	MsgProviderConnectRequest:   {},
	MsgProviderHeartbeatRequest: {},
	MsgProviderDrainRequest:     {},
}

// IsControlRequest reports whether the MsgID is a session/transport control
// request that must bypass the business dispatch lane.
func IsControlRequest(msgID uint32) bool {
	_, ok := controlRequests[msgID]
	return ok
}

// IsResponse returns true if the MsgID indicates a response message.
func IsResponse(msgID uint32) bool {
	return msgID%2 == 0 && msgID != MsgTaskEvent // TaskEvent is a stream event, not a response
}

// GetResponseMsgID returns the response MsgID for a given request MsgID.
func GetResponseMsgID(reqMsgID uint32) uint32 {
	return reqMsgID + 1
}

// MsgIDString returns a human-readable string representation of the MsgID.
func MsgIDString(msgID uint32) string {
	switch msgID {
	case MsgRegisterRequest:
		return "RegisterRequest"
	case MsgRegisterResponse:
		return "RegisterResponse"
	case MsgHeartbeatRequest:
		return "HeartbeatRequest"
	case MsgHeartbeatResponse:
		return "HeartbeatResponse"
	case MsgRegisterCapabilitiesReq:
		return "RegisterCapabilitiesRequest"
	case MsgRegisterCapabilitiesResp:
		return "RegisterCapabilitiesResponse"
	case MsgRegisterClientRequest:
		return "RegisterClientRequest"
	case MsgRegisterClientResponse:
		return "RegisterClientResponse"
	case MsgClientHeartbeatRequest:
		return "ClientHeartbeatRequest"
	case MsgClientHeartbeatResponse:
		return "ClientHeartbeatResponse"
	case MsgListClientsRequest:
		return "ListClientsRequest"
	case MsgListClientsResponse:
		return "ListClientsResponse"
	case MsgInvokeRequest:
		return "InvokeRequest"
	case MsgInvokeResponse:
		return "InvokeResponse"
	case MsgStartTaskRequest:
		return "StartTaskRequest"
	case MsgStartTaskResponse:
		return "StartTaskResponse"
	case MsgStreamTaskRequest:
		return "StreamTaskRequest"
	case MsgTaskEvent:
		return "TaskEvent"
	case MsgCancelTaskRequest:
		return "CancelTaskRequest"
	case MsgCancelTaskResponse:
		return "CancelTaskResponse"
	case MsgProviderConnectRequest:
		return "ProviderConnectRequest"
	case MsgProviderConnectResponse:
		return "ProviderConnectResponse"
	case MsgProviderHeartbeatRequest:
		return "ProviderHeartbeatRequest"
	case MsgProviderHeartbeatResponse:
		return "ProviderHeartbeatResponse"
	case MsgProviderDrainRequest:
		return "ProviderDrainRequest"
	case MsgProviderDrainResponse:
		return "ProviderDrainResponse"
	case MsgGetTaskResultRequest:
		return "GetTaskResultRequest"
	case MsgGetTaskResultResponse:
		return "GetTaskResultResponse"
	default:
		return fmt.Sprintf("Unknown(0x%06X)", msgID)
	}
}
