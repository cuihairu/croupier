# VS Code 配置文件

本目录包含 Croupier 项目的 VS Code 配置。

## 📁 文件说明

| 文件 | 用途 |
|------|------|
| `launch.json` | 调试启动配置 |
| `settings.json` | 项目设置 |
| `tasks.json` | 任务配置 |
| `extensions.json` | 推荐扩展 |
| `AGENT-DEBUG-GUIDE.md` | Agent 调试指南 |

## 🚀 快速开始

### 1. 安装推荐扩展

VS Code 会自动提示安装 `.vscode/extensions.json` 中的推荐扩展。

主要包括：
- Go (golang.go)
- YAML (redhat.vscode-yaml)
- GitLens (eamodio.gitlens)

### 2. 配置调试

按 `F5` 或点击调试面板，选择以下配置之一：

#### Server 配置
- **Server (dev sqlite)** - 使用 SQLite 数据库
- **Server (postgres)** - 使用 PostgreSQL
- **Server (mysql)** - 使用 MySQL

#### Agent 配置
- **Agent (多文件示例)** ⭐ - 加载多个 OpenAPI 文件
- **Agent (加载所有 Packs)** - 加载所有 Packs
- **Agent (调试模式)** - Debug 模式，可设置断点
- **Agent (微服务架构)** 🎯 - 多服务配置

详细说明请查看 [AGENT-DEBUG-GUIDE.md](./AGENT-DEBUG-GUIDE.md)

### 3. 运行任务

按 `Cmd+Shift+P` (Mac) 或 `Ctrl+Shift+P` (Windows/Linux)，输入 "Tasks: Run Task"，选择要运行的任务。

可用任务：
- `make: build` - 构建项目
- `make: test` - 运行测试
- `make: proto` - 生成 proto 代码
- `make: pack` - 生成 pack 文件

## 📝 项目设置

`settings.json` 包含以下配置：

```json
{
  // Go 配置
  "go.useLanguageServer": true,
  "go.toolsManagement.autoUpdate": true,

  // YAML 配置
  "yaml.format.enable": true,
  "yaml.validate": true,

  // 文件配置
  "files.eol": "\n",
  "files.trimTrailingWhitespace": true,
  "files.insertFinalNewline": true,

  // 代码格式化
  "editor.formatOnSave": true,
  "editor.codeActionsOnSave": {
    "source.organizeImports": "explicit"
  }
}
```

## 🔍 调试技巧

### 1. 设置断点

在任何 `.go` 文件中点击行号左侧设置断点。

### 2. 查看变量

调试时将鼠标悬停在变量上查看其值。

### 3. 调用栈

在调试左侧面板查看调用栈。

### 4. 日志输出

在 "DEBUG CONSOLE" 中查看程序输出。

## 🎯 常用操作

### 启动 Server + Agent

1. 先启动 Server：
   - 按 F5 → 选择 "Server (dev sqlite)"

2. 再启动 Agent：
   - 按 F5 → 选择 "Agent (多文件示例)"

### 调试 OpenAPI 解析

1. 在 `internal/platform/openapi/provider.go` 设置断点
2. 按 F5 → 选择 "Agent (调试模式)"
3. 触发 OpenAPI 加载

### 测试微服务架构

1. 启动各个微服务（8081, 8082, 8083...）
2. 按 F5 → 选择 "Agent (微服务架构)"
3. 测试函数调用

## 📚 更多文档

- [Agent 调试指南](./AGENT-DEBUG-GUIDE.md)
- [快速开始](../services/agent/etc/QUICKSTART.md)
- [完整指南](../services/agent/etc/README-OPENAPI.md)
- [多服务配置](../services/agent/etc/MULTI-SERVICE-GUIDE.md)

---

**最后更新**: 2024-02-07
