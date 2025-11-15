# 🔥 Croupier Node.js SDK 热重载示例

这个示例展示了如何在Node.js游戏服务器中集成Croupier SDK的热重载功能。

## 🚀 快速开始

### 1. 安装依赖

```bash
cd examples/js-hotreload
npm install
```

### 2. 启动Croupier Agent

```bash
# 在另一个终端启动Agent
cd ../../
make build
./bin/croupier-agent --config configs/agent.example.yaml
```

### 3. 选择运行方式

#### Nodemon开发模式（推荐）
```bash
npm run dev
```

#### PM2生产模式
```bash
npm run dev:pm2    # 开发环境
npm run prod       # 生产环境
```

#### 直接运行
```bash
npm start
```

## 🔧 热重载特性

### 1. 自动重连机制

当Nodemon重启Node.js进程时，SDK会：
- 检测连接断开
- 自动重连到Agent
- 重新注册所有函数
- 恢复正常服务

### 2. 模块热替换

```javascript
// 启用模块缓存清除
config.tools.moduleReload = true;

// 当文件变化时，自动清除require缓存
// 无需重启进程即可加载新代码
```

### 3. 文件监听

```javascript
config.fileWatching = {
  enabled: true,
  watchDir: './functions',
  patterns: ['*.js', '*.json']
};
```

### 4. 函数热重载

**单函数重载**：
```javascript
const newDescriptor = { id: 'player.ban', version: '1.1.0' };
await client.reloadFunction('player.ban', newDescriptor, newHandler);
```

**批量重载**：
```javascript
const functions = {
  'player.ban': { descriptor, handler },
  'server.status': { descriptor, handler }
};
await client.reloadFunctions(functions);
```

## 📊 开发工具集成

### Nodemon配置

```json
{
  "watch": ["main.js", "src/", "functions/"],
  "ext": "js,json",
  "env": {
    "NODE_ENV": "development",
    "CROUPIER_HOTRELOAD": "true"
  }
}
```

特性：
- 📁 监听多个目录
- 🔄 检测js/json文件变更
- 🚀 自动重启进程
- 🔗 SDK自动重连

### PM2配置

```bash
# 开发环境
npm run dev:pm2

# 生产环境
npm run prod

# 查看日志
npm run logs

# 热重载（零停机）
npm run reload
```

PM2特性：
- 🔄 零停机重载
- 📊 进程监控
- 📝 日志管理
- 🚀 集群模式

## 🎯 功能演示

运行后会自动演示：

1. **基础连接**（启动时）
   - 连接到Agent
   - 注册函数
   - 开始服务

2. **函数重载**（10秒后）
   - 将`player.ban`升级到v1.1.0
   - 增强功能特性

3. **批量重载**（20秒后）
   - 更新`server.status`到v2.0.0
   - 增加详细的系统信息

4. **状态监控**（每30秒）
   - 连接状态
   - 重载计数
   - 系统运行时间

## 🛠️ 开发工作流

### 修改函数逻辑

1. 编辑`main.js`中的处理函数
2. Nodemon检测文件变更
3. 自动重启进程
4. SDK自动重连并注册函数

### 测试API调用

```bash
# 测试玩家封禁
curl -X POST http://localhost:8080/api/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "player.ban",
    "payload": "{\"player_id\":\"123\",\"reason\":\"cheating\"}"
  }'

# 测试服务器状态
curl -X POST http://localhost:8080/api/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "server.status",
    "payload": "{}"
  }'
```

### 监控重载状态

热重载状态每30秒打印一次：
```
📊 Hot Reload Status:
  Connection: connected
  Reconnects: 2
  Function reloads: 3
  Failed reloads: 0
  Uptime: 125s
```

## 🎮 不同运行模式对比

| 模式 | 重载方式 | 停机时间 | 适用场景 | 命令 |
|------|---------|----------|----------|------|
| **Nodemon** | 进程重启 | ~1-2秒 | 开发环境 | `npm run dev` |
| **PM2 Dev** | 进程重启 | ~1秒 | 开发测试 | `npm run dev:pm2` |
| **PM2 Prod** | 零停机重载 | 0秒 | 生产环境 | `npm run prod` |
| **直接运行** | 手动重启 | N/A | 调试模式 | `npm start` |

## 🔍 调试和日志

### Nodemon日志
```bash
npm run dev
# 显示文件变更和重启信息
```

### PM2日志
```bash
npm run logs
# 查看所有进程日志

pm2 logs croupier-game --lines 100
# 查看特定进程日志
```

### 调试模式
```bash
NODE_ENV=development DEBUG=croupier:* npm run dev
# 启用详细调试日志
```

## 🚨 故障排除

### 常见问题

1. **模块缓存问题**
   ```
   Function not updated after reload
   ```
   - 确认`moduleReload: true`
   - 检查文件监听配置
   - 手动清除缓存

2. **连接失败**
   ```
   Connection failed
   ```
   - 检查Agent是否运行
   - 确认端口19090可用
   - 检查网络连接

3. **Nodemon无法启动**
   ```
   'nodemon' is not recognized
   ```
   - 全局安装：`npm install -g nodemon`
   - 或使用：`npx nodemon main.js`

### 最佳实践

1. **开发环境**
   - 使用Nodemon进行快速迭代
   - 启用文件监听和模块重载
   - 保持详细日志输出

2. **生产环境**
   - 使用PM2集群模式
   - 启用零停机重载
   - 配置日志轮转
   - 关闭开发特性

3. **测试环境**
   - 模拟生产配置
   - 测试重载功能
   - 验证连接恢复

## 📚 相关文档

- [SDK热重载支持文档](../../docs/SDK_HOTRELOAD_SUPPORT.md)
- [热更新方案总览](../../docs/HOT_RELOAD_SOLUTIONS.md)
- [Croupier架构说明](../../README.md)

---

*🔥 享受无缝的Node.js热重载开发体验！*