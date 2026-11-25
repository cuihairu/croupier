// Package labels defines agent registration label keys shared across
// components. Labels are self-reported by the agent at registration
// (RegisterRequest.labels); the server normalizes them via sanitizeAgentLabels
// and persists them into agent_sessions.labels. Any endpoint reading the same
// label must use the constants here; scattered bare string literals are
// forbidden (key drift = silent mismatch).
package labels

// AgentBinarySHA256 is the registration label key for the sha256 (lowercase
// hex) of the agent's running binary. §10.5 persistent fingerprint anchor: the
// agent self-reports it at each registration (runtime instance fingerprint,
// cached once per process lifetime), and it is persisted along with the
// registration labels — the system-info data plane relies on it to answer
// "last known fingerprint" after the agent goes offline or across server
// restarts.
const AgentBinarySHA256 = "agentBinarySha256"
