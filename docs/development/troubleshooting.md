---
title: 故障排查
---

# 🚨 Dashboard 看不到函数 - 排查指南

## 问题：Dashboard 后台看不到注册的函数

### ❌ 常见原因
1. **Server 未启动** - Agent 无法连接
2. **Agent 未启动** - 函数未注册
3. **函数同步失败** - Agent 到 Server 的同步失败

---

## ✅ 完整启动流程

### 步骤 1: 启动 Server（必须先启动！）

**在 VS Code 中**：
1. 按 `F5`
2. 选择 **"Server (dev sqlite)"** ⭐
3. 点击绿色播放按钮

**或命令行**：
```bash
cd /Users/cui/Workspaces/croupier/croupier
./bin/croupier-server -f services/server/etc/server.yaml
```

**验证 Server 启动成功**：
```bash
# 查看 Server 是否监听端口 8888
lsof -i :8888

# 或访问健康检查
curl http://localhost:8888/health
```

**期望输出**：
```
Starting api server at 0.0.0.0:8888...
```

---

### 步骤 2: 启动 Agent（Server 启动后）

**在 VS Code 中**：
1. 按 `F5`
2. 选择 **"Agent (多文件示例)"** ⭐
3. 点击绿色播放按钮

**或命令行**：
```bash
cd /Users/cui/Workspaces/croupier/croupier/services/agent
../../bin/croupier-agent -f etc/agent.yaml
```

**验证 Agent 启动成功**：
```bash
# 查看 Agent 是否监听端口 18888
lsof -i :18888

# 或查看日志
tail -f /var/log/croupier-agent.log
```

**期望输出**：
```
INFO  loading platform config from platforms.yaml
INFO  platform loaded name=examples methods=13
INFO  registering platform method function=examples.player.create
INFO  platform loaded name=packs methods=6
INFO  registering platform method function=packs.http.generic_invoke
...
```

---

### 步骤 3: 验证函数已注册到 Server

**检查 Server 的函数列表**：
```bash
# 查询 Server 上的所有函数
curl http://localhost:8888/api/v1/functions/descriptors | jq '.[] | .id' | head -20
```

**期望输出**：
```
examples.player.create
examples.player.get
examples.player.delete
...
packs.http.generic_invoke
packs.prom.query
...
```

**如果看不到函数**，说明 Agent 到 Server 的同步失败了。

---

### 步骤 4: 打开 Dashboard

1. 启动 Dashboard（如果还没启动）：
```bash
cd /Users/cui/Workspaces/croupier/croupier-dashboard
npm run dev
```

2. 打开浏览器访问：
```
http://localhost:8000
```

3. 登录后进入：
```
游戏管理 → 函数管理 → 函数目录
```

**现在应该能看到所有注册的函数了！**

---

## 🔍 详细排查步骤

### 检查 1: 确认 Server 在运行

```bash
# 检查端口
lsof -i :8888

# 如果没有输出，说明 Server 未启动
# 请先启动 Server（步骤 1）
```

### 检查 2: 确认 Agent 在运行

```bash
# 检查端口
lsof -i :18888

# 如果没有输出，说明 Agent 未启动
# 请启动 Agent（步骤 2）
```

### 检查 3: 查看 Agent 日志

**在 VS Code 中**：
- 启动 Agent (多文件示例) 后
- 在 "DEBUG CONSOLE" 查看日志

**或命令行查看**：
```bash
# 如果使用命令行启动
tail -f /var/log/croupier-agent.log

# 或查看实时日志
journalctl -u croupier-agent -f
```

**查找关键日志**：
```
✅ 成功标志：
  INFO platform loaded name=examples methods=13
  INFO registering platform method function=examples.player.create
  INFO syncing to upstream...

❌ 失败标志：
  ERROR failed to load platform config
  ERROR failed to connect to server
  ERROR upstream sync failed
```

### 检查 4: 测试 Server API

```bash
# 测试 Server 是否响应
curl http://localhost:8888/health

# 测试函数列表 API
curl http://localhost:8888/api/v1/functions/descriptors | jq 'length'

# 应该返回函数数量（如 19）
```

### 检查 5: 验证 platforms.yaml 配置

```bash
# 检查配置文件是否存在
cat /Users/cui/Workspaces/croupier/croupier/services/agent/etc/platforms.yaml

# 验证 YAML 语法
python3 -c "import yaml; yaml.safe_load(open('/Users/cui/Workspaces/croupier/croupier/services/agent/etc/platforms.yaml'))"

# 检查 OpenAPI 文件是否存在
ls -la /Users/cui/Workspaces/croupier/croupier/services/agent/etc/openapi.example.yaml
```

---

## 🐛 常见问题和解决方案

### 问题 1: Agent 连接 Server 失败

**错误信息**：
```
ERROR failed to connect to server: dial tcp :19090: connect: connection refused
```

**原因**：Server 未启动

**解决**：
1. 先启动 Server（步骤 1）
2. 等待 Server 完全启动（约 2-3 秒）
3. 再启动 Agent（步骤 2）

---

### 问题 2: 函数未同步到 Server

**现象**：Agent 启动成功，但 Server 上没有函数

**检查 Agent 日志**：
```
# 查找 "syncing to upstream" 或 "upstream sync" 日志
tail -f /var/log/croupier-agent.log | grep -i sync
```

**可能原因**：
- Server 的 Upstream API 不可用
- Agent 配置中的 Upstream 地址错误

**验证**：
```bash
# 测试 Server 的 Upstream 端口
curl http://localhost:19090/
```

---

### 问题 3: platforms.yaml 配置错误

**检查配置**：
```bash
# 验证 YAML 语法
cd /Users/cui/Workspaces/croupier/croupier/services/agent/etc
python3 -c "import yaml; yaml.safe_load(open('platforms.yaml'))"

# 查看配置内容
cat platforms.yaml
```

**确保**：
- `enabled: true` 已设置
- OpenAPI 文件路径正确
- `base_url` 配置正确

---

### 问题 4: Dashboard 显示空白

**检查**：
1. Dashboard 是否成功启动
2. 浏览器控制台是否有错误
3. Server API 是否可访问

**测试 Dashboard 连接**：
```bash
# 测试 Dashboard 能否访问 Server
curl http://localhost:8888/api/v1/functions/descriptors
```

---

## 📊 验证成功的标志

### ✅ 启动成功的标志

**Server 日志**：
```
Starting api server at 0.0.0.0:8888...
Server is ready.
```

**Agent 日志**：
```
INFO  loading platform config from platforms.yaml
INFO  platform loaded name=examples methods=13
INFO  platform loaded name=packs methods=6
INFO  registering platform method function=examples.player.create
...
INFO  syncing to upstream...
```

**Dashboard 函数目录**：
```
函数列表：
  examples.player.create (玩家创建)
  examples.player.get (玩家查询)
  examples.player.delete (玩家删除)
  examples.player.ban_batch (批量封禁)
  examples.inventory.get (获取背包)
  ...
  packs.http.generic_invoke (HTTP 调用)
  packs.prom.query (Prometheus 查询)
  ...
```

**总共 19 个函数**（13 示例 + 6 Packs）

---

## 🎯 快速启动命令（一键式）

### 使用 VS Code（推荐）

1. **启动 Server**：
   - 按 `F5` → 选择 "Server (dev sqlite)"
   - 等待 2-3 秒

2. **启动 Agent**：
   - 再次按 `F5` → 选择 "Agent (多文件示例)"

3. **打开 Dashboard**：
   - 浏览器访问 `http://localhost:8000`
   - 游戏管理 → 函数管理 → 函数目录

### 使用命令行

```bash
# Terminal 1: 启动 Server
cd /Users/cui/Workspaces/croupier/croupier
./bin/croupier-server -f services/server/etc/server.yaml

# Terminal 2: 启动 Agent
cd /Users/cui/Workspaces/croupier/croupier/services/agent
../../bin/croupier-agent -f etc/agent.yaml

# Terminal 3: 启动 Dashboard（可选）
cd /Users/cui/Workspaces/croupier/croupier-dashboard
npm run dev
```

---

## 💡 关键要点

### 启动顺序很重要！

**正确的顺序**：
```
1. Server ← 必须先启动！
2. Agent ← Server 启动后再启动
3. Dashboard ← 最后启动
```

**错误**：
```
❌ 先启动 Agent → 无法连接 Server
❌ 只启动 Dashboard → Server 和 Agent 都没启动
```

### 验证每个步骤

```bash
# 1. 验证 Server
lsof -i :8888  # 应该有输出

# 2. 验证 Agent
lsof -i :18888 # 应该有输出

# 3. 验证函数注册
curl http://localhost:8888/api/v1/functions/descriptors | jq 'length'
# 应该返回 > 0
```

---

## 📚 相关文档

- [VS Code 启动配置](.vscode/LAUNCH-CONFIGS.md)
- [Agent 配置指南](services/agent/etc/README-OPENAPI.md)
- [函数管理快速参考](croupier-dashboard/QUICK-REFERENCE.md)

---

**最后更新**: 2024-02-07
**快速链接**：
- 📋 函数目录: http://localhost:8000/game/functions/catalog
- 🔧 Server 配置: services/server/etc/server.yaml
- 🔧 Agent 配置: services/agent/etc/agent.yaml
