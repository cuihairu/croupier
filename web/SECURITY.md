# Security Policy

## Supported Versions

当前 Croupier Dashboard 以下版本获得安全更新支持：

| Version | Supported          |
| ------- | ------------------ |
| 6.x     | :white_check_mark: |
| < 6.0   | :x:                |

## Reporting a Vulnerability

如果您发现安全漏洞，请通过以下方式负责任地披露：

1. **请勿** 在公开的 GitHub Issues 中报告安全漏洞
2. 发送邮件至项目维护者，或通过 [GitHub Security Advisories](https://github.com/cuihairu/croupier-dashboard/security/advisories/new) 提交报告
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

使用 Croupier Dashboard 时，建议遵循以下安全实践：

- 始终使用最新稳定版本
- 定期更新前端依赖项（npm audit）
- 在生产环境中启用 HTTPS
- 配置合适的 CSP（Content Security Policy）
- 妥善保管 API 密钥和认证凭据

## Known Vulnerabilities

以下漏洞来自间接依赖（transitive dependencies），暂时无法直接修复：

### 1. mockjs (高危 - GHSA-mh8j-9jvh-gjf6)

- **影响**: 原型污染漏洞 (CVE-2023-26158)
- **状态**: 直接依赖，但无官方补丁版本
- **缓解措施**: 用户输入不应直接传递给 mockjs 的 extend() 方法
- **修复计划**: 评估替换为维护活跃的 fork（如 `mockjs-plus`）或其他模拟库

### 2. request (中危 - GHSA-p8p7-x288-28g6)

- **影响**: 服务端请求伪造 (SSRF)
- **路径**: `@ant-design/pro-cli > pngjs-image > request`
- **状态**: 间接依赖，已过时但无法直接更新
- **缓解措施**: 仅在开发环境使用 @ant-design/pro-cli，不影响生产环境

### 3. elliptic (低危 - GHSA-848j-6mx2-7j84)

- **影响**: 加密原语实现问题
- **路径**: `@umijs/max > umi > @umijs/bundler-webpack > node-libs-browser > crypto-browserify > browserify-sign > elliptic`
- **状态**: 深层间接依赖，风险较低
- **缓解措施**: 不直接影响应用安全性，仅用于开发构建

### 修复措施

- ✅ 已通过 pnpm overrides 强制所有 eslint 依赖使用 >=9.26.0（修复堆栈溢出漏洞）
- 🔄 持续监控上游依赖更新，一旦有可用修复将立即更新
