package tlsutil

// ClientTLSConfig carries plain TLS dial settings for internal transports.
// It intentionally has no dependency on any RPC framework: gRPC was removed
// from this project (see docs/architecture/transport-no-grpc.md), and the
// former grpc/credentials helpers here were dead code from that era.

type ClientTLSConfig struct {
	CertFile           string
	KeyFile            string
	CAFile             string
	ServerName         string
	InsecureSkipVerify bool
}
