package cluster

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
)

var errPeerIdentity = errors.New("interconnect: peer certificate identity invalid")

// VerifyInterconnectPeer is the mTLS VerifyConnection hook shared by the
// interconnect TLS client and server. It accepts only "server" role
// certificates: a peer must present a CA-signed certificate with both
// ServerAuth and ClientAuth extended key usages and a non-empty CommonName.
// Agent certificates carry only ClientAuth and are rejected, satisfying the
// ARCH-003 "Agent identity rejected" requirement. The CA chain itself is
// verified by the standard TLS verification (RootCAs/ClientCAs).
func VerifyInterconnectPeer(state tls.ConnectionState) error {
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("%w: no peer certificate", errPeerIdentity)
	}
	cert := state.PeerCertificates[0]
	if cert.Subject.CommonName == "" {
		return fmt.Errorf("%w: peer CommonName empty", errPeerIdentity)
	}
	if !hasServerRole(cert) {
		return fmt.Errorf("%w: peer is not a server role certificate", errPeerIdentity)
	}
	return nil
}

func hasServerRole(cert *x509.Certificate) bool {
	var serverAuth, clientAuth bool
	for _, usage := range cert.ExtKeyUsage {
		switch usage {
		case x509.ExtKeyUsageServerAuth:
			serverAuth = true
		case x509.ExtKeyUsageClientAuth:
			clientAuth = true
		}
	}
	return serverAuth && clientAuth
}
