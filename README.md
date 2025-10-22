# Croupier - 游戏GM后台系统

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.21+-green.svg)
![Status](https://img.shields.io/badge/status-in%20development-yellow.svg)

Croupier 是一个专为游戏运营设计的通用GM后台系统，支持多语言游戏服务器接入，提供统一的管理界面和强大的扩展能力。

## 🎯 核心特性

- **🔌 极简SDK接入**: 3行代码完成游戏服务器集成
- **🌐 多语言支持**: Go、Java、C++、Python等语言SDK
- **🔒 安全可靠**: 基于TCP的安全连接和身份验证
- **🎨 自动化UI**: 根据功能注册自动生成管理界面
- **📊 实时数据**: 支持实时数据展示和操作执行
- **🔧 灵活部署**: 支持直连和代理部署模式，适应不同网络拓扑

## 🏗️ 系统架构

### 整体架构图

```
                    ┌─── Web管理界面 ───┐
                    │                  │
              ┌─────────────────────────────────┐
              │        Croupier Core            │
              │  ┌─────────────────────────────┐ │
              │  │     权限控制 + UI生成        │ │
              │  └─────────────────────────────┘ │
              │  ┌─────────────────────────────┐ │
              │  │     功能调用引擎            │ │
              │  └─────────────────────────────┘ │
              │  ┌─────────────────────────────┐ │
              │  │   TCP连接管理 + 认证        │ │
              │  └─────────────────────────────┘ │
              └─────────────────────────────────┘
                         ▲
                         │ TCP连接 (端口:9090)
              ┌──────────┼──────────┐
              │          │          │
        ┌─────────┐ ┌─────────┐ ┌─────────┐
        │   Go    │ │  Java   │ │   C++   │
        │ Server  │ │ Server  │ │ Server  │
        │         │ │         │ │         │
        │ ┌─────┐ │ │ ┌─────┐ │ │ ┌─────┐ │
        │ │ SDK │ │ │ │ SDK │ │ │ │ SDK │ │
        │ └─────┘ │ │ └─────┘ │ │ └─────┘ │
        └─────────┘ └─────────┘ └─────────┘
```

### 部署模式设计

#### 模式1: 直连部署 (同网段)
```
┌─────────────────────────────────────────┐
│              游戏内网                    │
│                                         │
│  ┌─────────────┐    ┌─────────────────┐ │
│  │   Croupier  │    │   Game Servers  │ │
│  │    Core     │◄───┤      + SDK      │ │
│  │             │    │                 │ │
│  └─────────────┘    └─────────────────┘ │
└─────────────────────────────────────────┘
```

#### 模式2: 代理部署 (跨网段)
```
┌──────────── DMZ ─────────────┐  ┌────────── 游戏内网 ──────────┐
│                              │  │                             │
│  ┌─────────────────────────┐ │  │  ┌─────────────────────────┐ │
│  │      Croupier Core      │ │  │  │   Croupier Proxy        │ │
│  │                         │ │  │  │   (轻量级转发)           │ │
│  └─────────────────────────┘ │  │  └─────────────────────────┘ │
│              ▲               │  │              ▲               │
│              │               │  │              │               │
│              └───────────────┼──┼──────────────┘               │
│                              │  │                             │
│                              │  │  ┌─────────────────────────┐ │
│                              │  │  │    Game Servers         │ │
│                              │  │  │       + SDK             │ │
│                              │  │  └─────────────────────────┘ │
└──────────────────────────────┘  └─────────────────────────────┘
```

## 🚀 快速开始

### 部署模式1: 直连部署 (推荐)

适用于所有服务在同一内网的情况

```bash
# 1. 启动Croupier Core
./croupier-server --config configs/croupier.yaml

# 2. 游戏服务器直接连接
# 配置: server_addr: "croupier:9090"
./game-server
```

### 部署模式2: 代理部署

适用于GM后台在DMZ，游戏服务器在内网的情况

```bash
# 1. DMZ启动Croupier Core
./croupier-server --config configs/croupier.yaml

# 2. 内网启动轻量级Proxy
./croupier-proxy --config configs/proxy.yaml

# 3. 游戏服务器连接Proxy
# 配置: server_addr: "proxy:9090"
./game-server
```

### 2. 游戏服务器集成

#### Go服务器
```go
package main

import "github.com/croupier/sdk-simple"

func main() {
    // 1. 创建SDK
    sdk := simple.NewSDK("configs/sdk.yaml")

    // 2. 注册功能
    sdk.RegisterFunction(simple.Function{
        ID:          "player.ban",
        Name:        "封禁玩家",
        Description: "根据玩家ID封禁玩家",
        Category:    "player",
        Permission:  "admin",
        Handler:     handlePlayerBan,
    })

    // 3. 连接服务
    sdk.Connect()

    // 保持运行
    select {}
}

func handlePlayerBan(data []byte) ([]byte, error) {
    // 处理封禁逻辑
    return []byte(`{"success": true}`), nil
}
```

#### Java服务器
```java
@CroupierService("analytics-service")
public class AnalyticsService {

    @CroupierFunction(
        id = "analytics.query",
        name = "数据查询",
        category = "analytics",
        permission = "operator"
    )
    public QueryResult handleQuery(@Param("query") String query) {
        // 处理查询逻辑
        return analyticsEngine.execute(query);
    }
}

// 启动
public static void main(String[] args) {
    CroupierSDK.start(AnalyticsService.class);
}
```

### 3. 使用GM后台

访问 `http://localhost:8080` 即可看到自动生成的管理界面。

## 📋 项目结构

```
croupier/
├── cmd/
│   ├── server/           # Croupier服务器主程序
│   └── cli/              # 命令行工具
├── internal/
│   ├── server/           # 服务器核心逻辑
│   ├── auth/             # 认证模块
│   ├── connection/       # 连接管理
│   ├── function/         # 功能调用引擎
│   └── web/              # Web界面后端
├── pkg/
│   ├── sdk-simple/       # 极简SDK
│   ├── protocol/         # TCP协议定义
│   └── types/            # 公共类型定义
├── web/                  # React前端项目
│   ├── src/
│   │   ├── components/   # UI组件
│   │   ├── pages/        # 页面
│   │   └── services/     # API服务
│   └── package.json
├── configs/              # 配置文件
├── scripts/              # 部署脚本
├── docs/                 # 文档
└── examples/             # 示例代码
    ├── go-server/
    ├── java-server/
    └── cpp-server/
```

## 🔐 安全设计

### 连接认证

#### 被动模式认证流程
1. **建立连接**: 游戏服务器连接到Croupier
2. **身份验证**: 使用HMAC-SHA256签名验证
3. **会话管理**: 颁发临时令牌，24小时有效
4. **功能注册**: 注册服务提供的功能列表

#### 认证消息格式
```json
{
  "service_id": "game-server-1",
  "api_key": "gs1_api_key_123",
  "timestamp": 1640995200,
  "signature": "HMAC-SHA256(service_id+timestamp, secret)"
}
```

### 权限控制

- **viewer**: 查看权限，只能查看数据
- **operator**: 操作权限，可执行一般GM操作
- **admin**: 管理权限，可执行所有操作

### 防攻击机制

- **时间戳验证**: 防重放攻击，5分钟有效期
- **连接限制**: 限制同一IP的连接数
- **操作审计**: 记录所有GM操作日志

## 🗓️ 开发计划

### Phase 1: 核心基础 (MVP) - 4周

#### Week 1: 基础架构
- [ ] TCP连接管理模块
- [ ] 简单身份验证
- [ ] 基础协议定义
- [ ] Go SDK实现

**交付物**:
- Croupier服务器可启动
- Go SDK可连接和注册功能
- 基础TCP通信正常

#### Week 2: 功能调用引擎
- [ ] 功能注册机制
- [ ] 调用路由和分发
- [ ] 参数验证和处理
- [ ] 错误处理和重试

**交付物**:
- 支持功能注册和调用
- 完整的错误处理机制
- 基础测试用例

#### Week 3: Web管理界面
- [ ] React前端项目搭建
- [ ] 自动UI生成组件
- [ ] 功能列表和分类展示
- [ ] 参数表单和执行界面

**交付物**:
- 可用的Web管理界面
- 支持查看和执行功能
- 基础的用户交互

#### Week 4: 权限和安全
- [ ] 用户认证和会话管理
- [ ] RBAC权限控制
- [ ] 操作审计日志
- [ ] 基础安全防护

**交付物**:
- 完整的权限控制系统
- 安全的GM操作环境
- MVP版本发布

### Phase 2: 多语言支持 - 3周

#### Week 5-6: SDK扩展
- [ ] Java SDK开发
- [ ] C++ SDK开发
- [ ] Python SDK开发
- [ ] SDK文档和示例

#### Week 7: 集成测试
- [ ] 多语言SDK集成测试
- [ ] 性能测试和优化
- [ ] 文档完善

### Phase 3: 高级特性 - 4周

#### Week 8-9: 高级功能
- [ ] 实时数据推送
- [ ] 批量操作支持
- [ ] 数据导出功能
- [ ] 自定义Dashboard

#### Week 10-11: 生产就绪
- [ ] 监控和告警
- [ ] 部署脚本和工具
- [ ] 性能优化
- [ ] 安全加固

### Phase 4: 企业特性 - 3周

#### Week 12-14: 企业功能
- [ ] 多租户支持
- [ ] 高可用部署
- [ ] 数据备份和恢复
- [ ] 企业级安全

## 🤝 贡献指南

我们欢迎所有形式的贡献！

### 开发环境搭建

```bash
# 1. 克隆项目
git clone https://github.com/your-org/croupier.git
cd croupier

# 2. 安装Go依赖
go mod download

# 3. 安装前端依赖
cd web && npm install

# 4. 启动开发服务器
make dev
```

### 提交流程

1. Fork项目
2. 创建功能分支: `git checkout -b feature/amazing-feature`
3. 提交更改: `git commit -m 'Add amazing feature'`
4. 推送分支: `git push origin feature/amazing-feature`
5. 提交Pull Request

## 📖 文档

- [API文档](docs/api.md)
- [SDK开发指南](docs/sdk-development.md)
- [部署指南](docs/deployment.md)
- [安全最佳实践](docs/security.md)

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 💬 社区

- [GitHub Issues](https://github.com/your-org/croupier/issues) - 问题反馈
- [GitHub Discussions](https://github.com/your-org/croupier/discussions) - 社区讨论
- [Wiki](https://github.com/your-org/croupier/wiki) - 详细文档

## 🙏 致谢

感谢所有为Croupier项目做出贡献的开发者和社区成员！

---

**Croupier** - 让游戏运营变得简单而强大 🎮