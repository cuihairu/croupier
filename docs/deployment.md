# Deployment

- Server runs in DMZ/public with mTLS on :443 (configurable via --config or env CROUPIER_*)
- Agent runs in game private networks and dials out to Server (--config or env supported)
- Game servers connect to local Agent

Status: skeleton.
