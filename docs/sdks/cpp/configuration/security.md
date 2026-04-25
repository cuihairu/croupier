# 安全配置

## 规则

- 本机或受控内网可以不开 TLS
- 跨主机、跨网段或合规要求场景启用 TLS
- 双向身份校验场景启用 mTLS

## 注意

- TLS 是 session 的安全配置，不是新的 transport kind
