# Security Policy

## Supported Versions

当前 Croupier Go SDK 正处于早期开发阶段，以下版本获得安全更新支持：

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

如果您发现安全漏洞，请通过以下方式负责任地披露：

1. **请勿** 在公开的 GitHub Issues 中报告安全漏洞
2. 发送邮件至项目维护者，或通过 [GitHub Security Advisories](https://github.com/cuihairu/croupier-sdk-go/security/advisories/new) 提交报告
3. 请在报告中包含：
   - 漏洞描述
   - 复现步骤
   - 潜在影响
   - 如有可能，提供修复建议

## Response Timeline

- **确认收到**：48 小时内
- **初步评估**：7 个工作日内
- **修复发布**：视漏洞严重程度而定，高危漏洞优先处理

## Security Best Practices

使用 Croupier Go SDK 时，建议遵循以下安全实践：

- 始终使用最新稳定版本
- 定期运行 `go mod tidy` 和 `govulncheck` 检查依赖漏洞
- 在生产环境中启用 TLS/mTLS 加密通信
- 使用环境变量或 Vault 等工具管理敏感配置
- 避免在日志中输出敏感信息
