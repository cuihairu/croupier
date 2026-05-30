# 环境配置

建议使用明确的 profile 概念区分运行环境：

- `local-dev`
- `intra-cluster`
- `cross-zone-tls`
- `cross-zone-mtls`

## 建议

- 开发环境允许明文
- 跨主机和合规环境启用 TLS 或 mTLS
