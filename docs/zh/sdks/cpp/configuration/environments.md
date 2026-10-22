# 环境分区

用显式 profile 区分运行环境：

- `local-dev`
- `intra-cluster`
- `cross-zone-tls`
- `cross-zone-mtls`

## 建议

- 开发环境可以明文
- 跨主机与合规敏感环境启用 TLS 或 mTLS
