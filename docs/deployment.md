# Deployment

- Server runs in DMZ/public with mTLS on :443 (configurable via --config or env CROUPIER_SERVER_*)
- Agent runs in game private networks and dials out to Server (--config or env CROUPIER_AGENT_*)
- Edge (optional) can be started with `croupier edge` to relay between Server and Agents
- Game servers connect to local Agent

Status: skeleton.
