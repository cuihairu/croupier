# Environments

Use explicit profiles to distinguish runtime environments:

- `local-dev`
- `intra-cluster`
- `cross-zone-tls`
- `cross-zone-mtls`

## Recommendations

- Plaintext is acceptable in development environments
- Enable TLS or mTLS for cross-host and compliance-sensitive environments
