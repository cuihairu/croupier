---
title: VSCode 开发环境设置
---

# VSCode 开发环境设置指南

本文档说明了Croupier项目中VSCode的推荐插件和设置配置。

## 🚀 必需插件

### Go / go-zero 开发
- **golang.go** - Go语言官方支持插件
- **goctl.vscode-goctl** - go-zero CLI工具集成，用于API代码生成和服务管理

### Protocol Buffers 支持
- **bufbuild.vscode-buf** - Buf Protocol Buffer工具集成
- **zxh404.vscode-proto3** - Protocol Buffers语法高亮和智能提示

### 多语言支持
- **vscjava.vscode-java-pack** - Java开发套件
- **ms-python.python** - Python开发支持
- **ms-vscode.cpptools** - C/C++开发支持

### 代码质量和格式化
- **dbaeumer.vscode-eslint** - JavaScript/TypeScript代码检查
- **esbenp.prettier-vscode** - 代码格式化工具

### 配置文件支持
- **redhat.vscode-yaml** - YAML文件语法高亮和验证
- **bradlc.vscode-tailwindcss** - Tailwind CSS支持

### 开发效率
- **formulahendry.auto-rename-tag** - 自动重命名成对标签

## 🔧 推荐配置

创建 `.vscode/settings.json` 文件：

```json
{
  "go.toolsManagement.checkForUpdates": "local",
  "go.useLanguageServer": true,
  "go.gopath": "",
  "go.goroot": "",
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace",
  "go.formatTool": "goimports",
  "editor.formatOnSave": true,
  "editor.codeActionsOnSave": {
    "source.organizeImports": true
  },
  "files.associations": {
    "*.api": "proto"
  },
  "files.exclude": {
    "**/node_modules": true,
    "**/dist": true,
    "**/.git": true
  }
}
```

## 🛠️ go-zero 开发工作流

### 1. API 开发
```bash
# 在 services/api 目录下
goctl api go -api api.api -dir .
```

### 2. RPC 服务开发
```bash
# 在 proto 目录下
buf build
buf generate
```

### 3. 代码生成
```bash
# 生成 API handler
goctl api handler -api api.api -dir .

# 生成 RPC 代码
goctl rpc protoc pb/*.proto --go_out=. --go-grpc_out=.
```

## 📝 使用技巧

### 1. 快速生成 API
- 使用快捷键 `Ctrl+Shift+P` 输入 "Goctl" 查看可用命令
- 在 `.api` 文件中使用 `Ctrl+Shift+P` 输入 "Goctl: Generate API"

### 2. Proto 文件编辑
- VSCode会自动识别 `.api` 和 `.proto` 文件
- Buf插件提供实时语法检查和格式化

### 3. 调试配置
创建 `.vscode/launch.json`：

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch API Server",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/services/api/api.go",
      "env": {
        "CROUPIER_SERVER_HOST": "0.0.0.0",
        "CROUPIER_SERVER_PORT": "8888"
      }
    },
    {
      "name": "Launch Agent",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/services/agent/agent.go",
      "env": {
        "CROUPIER_AGENT_ID": "agent-1",
        "CROUPIER_AGENT_GAME_ID": "demo-game"
      }
    }
  ]
}
```

## 🎯 项目特定设置

### Go Module 代理
```bash
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=sum.golang.google.cn
```

### 工作区配置
```json
{
  "go.testFlags": ["-v"],
  "go.buildFlags": ["-v"],
  "go.testTimeout": "30s",
  "go.coverOnSave": true,
  "go.coverageDecorator": {
    "type": "gutter",
    "coveredHighlightColor": "rgba(64,128,64,0.5)",
    "uncoveredHighlightColor": "rgba(128,64,64,0.25)"
  }
}
```

## 📋 开发检查清单

- [ ] 安装所有推荐插件
- [ ] 配置Go语言环境
- [ ] 安装goctl CLI工具
- [ ] 配置工作区设置
- [ ] 创建调试配置
- [ ] 测试API生成功能
- [ ] 验证Proto文件支持

## 🔗 相关链接

- [go-zero 官方文档](https://go-zero.dev/)
- [Go 官方插件文档](https://github.com/golang/vscode-go)
- [Buf VSCode插件](https://buf.build/blog/vscode-plugin)
- [goctl VSCode插件](https://github.com/zeromicro/goctl)

---

**注意**: 首次使用时，VSCode会提示安装推荐的插件，请确保全部安装以获得最佳开发体验。