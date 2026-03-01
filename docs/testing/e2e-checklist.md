# 函数注册到 Dashboard 展示 - 端到端验收清单

本文档提供 SDK → Agent → Server → Dashboard 完整链路的联调测试清单与验收标准。

## 测试环境准备

### 前置条件
- [ ] Server 已启动（默认 8080 HTTP，8443 gRPC）
- [ ] Agent 已启动并成功连接 Server
- [ ] Dashboard 已启动（默认 8000 端口）
- [ ] 测试游戏 ID 已配置（如 `test-game`）

### 验证服务可用性
```bash
# 检查 Server
curl http://localhost:8080/healthz

# 检查 Agent 连接状态
curl http://localhost:8080/api/v1/agents

# 检查 Dashboard
curl http://localhost:8000
```

---

## 阶段一：SDK → Agent 注册

### 1.1 函数注册请求

**测试用例：游戏服务器通过 SDK 注册函数**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 游戏服务器启动 SDK 并注册函数 | Agent 返回注册成功响应 |
| 2 | 查询 Agent 本地函数列表 | 新注册函数出现在列表中 |
| 3 | 函数包含完整描述 | name, description, category 等字段完整 |

**验证命令：**
```bash
# 通过 Agent gRPC API 查询已注册函数
grpcurl -plaintext localhost:19090 list.FunctionRegistry/ListFunctions

# 预期响应包含：
# {
#   "functions": [
#     {
#       "id": "test.function.v1",
#       "name": "测试函数",
#       "category": "player",
#       "description": "用于测试的函数"
#     }
#   ]
# }
```

### 1.2 OpenAPI 描述生成

**测试用例：函数自动生成 OpenAPI 描述**

| 检查项 | 验证点 |
|--------|--------|
| Operation ID | 与函数 ID 一致 |
| Request Schema | 包含所有参数的 JSON Schema |
| x-ui 扩展 | 包含 UI 渲染提示（如有） |
| Response Schema | 包含返回值结构定义 |

---

## 阶段二：Agent → Server 同步

### 2.1 函数同步到 Server

**测试用例：Agent 向 Server 同步函数注册**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | Agent 连接 Server 后自动同步 | Server 返回同步成功 |
| 2 | 查询 Server 函数列表 | 新函数出现在 Registry 中 |
| 3 | 查询函数 OpenAPI | OpenAPI Operation 完整可用 |

**验证命令：**
```bash
# 查询 Server 上的函数列表
curl http://localhost:8080/api/v1/functions

# 预期响应包含：
# {
#   "functions": [
#     {
#       "id": "test.function.v1",
#       "name": "测试函数",
#       "category": "player",
#       "status": "active"
#     }
#   ]
# }

# 查询函数 OpenAPI
curl http://localhost:8080/api/v1/functions/test.function.v1/openapi
```

### 2.2 Descriptor 生成

**测试用例：Server 生成函数 Descriptor**

| 检查项 | 验证点 |
|--------|--------|
| 函数 ID | 与注册时一致 |
| 显示名称 | 使用函数 name 或友好名称 |
| 分类 | 与 category 字段一致 |
| 菜单配置 | menu.hidden, menu.order 正确设置 |
| 路由配置 | path, section, group 合理分配 |

**验证命令：**
```bash
# 查询函数 Descriptors
curl http://localhost:8080/api/v1/functions/descriptors

# 预期响应包含：
# {
#   "descriptors": [
#     {
#       "id": "test.function.v1",
#       "name": "测试函数",
#       "category": "player",
#       "menu": {
#         "hidden": false,
#         "order": 100
#       },
#       "route": {
#         "path": "/game/functions/registered/test-function-v1",
#         "section": "game",
#         "group": "Registered"
#       }
#     }
#   ]
# }
```

---

## 阶段三：Dashboard 展示

### 3.1 菜单动态生成

**测试用例：Dashboard 根据Descriptors 生成菜单**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | Dashboard 启动 | 调用 /api/v1/functions/descriptors |
| 2 | 左侧菜单渲染 | "Game" → "Registered" 分组出现 |
| 3 | 点击分组 | 新注册函数显示在列表中 |
| 4 | 菜单顺序 | 按 menu.order 正确排序 |

**UI 验证点：**
- [ ] 函数显示在正确的菜单分组中
- [ ] 函数名称与描述正确展示
- [ ] 图标（如有）正确显示
- [ ] 隐藏函数（hidden=true）不显示

### 3.2 函数详情页

**测试用例：点击函数进入详情页**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 点击函数菜单项 | 跳转到函数详情页 |
| 2 | 详情页加载 | 显示函数元信息 |
| 3 | OpenAPI 请求 | 自动调用 /api/v1/functions/{id}/openapi |
| 4 | 表单渲染 | Formily 根据 Schema 渲染输入表单 |

**验证字段：**
- [ ] 函数 ID、名称、描述
- [ ] 所属分类
- [ ] 参数列表及类型
- [ ] 返回值结构
- [ ] 调用按钮

---

## 阶段四：UI 配置编辑

### 4.1 读取 UI 配置

**测试用例：查看函数 UI 配置**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 进入函数详情页 | 点击"UI 配置"标签 |
| 2 | 查看当前配置 | 显示 JSON 格式 UI 配置 |
| 3 | 配置来源 | 显示 x-ui 或自定义配置 |

**验证命令：**
```bash
# 获取函数 UI 配置
curl http://localhost:8080/api/v1/functions/test.function.v1/ui

# 预期响应包含：
# {
#   "function_id": "test.function.v1",
#   "config": {
#     "layout": "vertical",
#     "labelWidth": 120,
#     "size": "middle"
#   },
#   "source": "x-ui",
#   "version": 1
# }
```

### 4.2 编辑 UI 配置

**测试用例：修改函数 UI 配置**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 修改配置项 | 修改 labelWidth, size 等字段 |
| 2 | 保存配置 | 调用 PUT /api/v1/functions/{id}/ui |
| 3 | 保存成功 | 显示成功提示 |
| 4 | 版本递增 | version 字段 +1 |

**验证命令：**
```bash
# 更新 UI 配置
curl -X PUT http://localhost:8080/api/v1/functions/test.function.v1/ui \
  -H "Content-Type: application/json" \
  -d '{
    "layout": "horizontal",
    "labelWidth": 150,
    "size": "large"
  }'

# 预期响应：
# {
#   "success": true,
#   "version": 2
# }
```

### 4.3 UI 历史记录

**测试用例：查看 UI 配置变更历史**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 点击"历史记录" | 调用 /api/v1/functions/{id}/ui/history |
| 2 | 显示历史列表 | 按时间倒序显示所有版本 |
| 3 | 查看版本详情 | 可展开查看每版本配置 |
| 4 | 版本对比 | 显示与当前版本的 diff |

**验证命令：**
```bash
# 获取 UI 历史记录
curl http://localhost:8080/api/v1/functions/test.function.v1/ui/history

# 预期响应包含：
# {
#   "history": [
#     {
#       "version": 2,
#       "created_at": "2026-03-01T10:30:00Z",
#       "created_by": "admin",
#       "config": { "layout": "horizontal", ... }
#     },
#     {
#       "version": 1,
#       "created_at": "2026-03-01T09:00:00Z",
#       "created_by": "system",
#       "config": { "layout": "vertical", ... }
#     }
#   ]
# }
```

### 4.4 UI 回滚

**测试用例：回滚到历史版本**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 选择历史版本 | 点击某个版本的"回滚"按钮 |
| 2 | 确认回滚 | 显示确认对话框 |
| 3 | 执行回滚 | 调用 POST /api/v1/functions/{id}/ui/rollback |
| 4 | 回滚成功 | 配置恢复，版本号 +1 |

**验证命令：**
```bash
# 回滚到指定版本
curl -X POST http://localhost:8080/api/v1/functions/test.function.v1/ui/rollback \
  -H "Content-Type: application/json" \
  -d '{ "version": 1 }'

# 预期响应：
# {
#   "success": true,
#   "version": 3,
#   "message": "已回滚到版本 1，新版本号为 3"
# }
```

---

## 阶段五：菜单路由编辑

### 5.1 读取路由配置

**测试用例：查看函数路由配置**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 进入函数详情页 | 点击"路由配置"标签 |
| 2 | 显示路由信息 | path, section, group, order, hidden |

**验证命令：**
```bash
# 获取路由配置
curl http://localhost:8080/api/v1/functions/test.function.v1/route

# 预期响应包含：
# {
#   "function_id": "test.function.v1",
#   "route": {
#     "path": "/game/functions/registered/test-function-v1",
#     "section": "game",
#     "group": "Registered",
#     "order": 100,
#     "hidden": false
#   }
# }
```

### 5.2 编辑路由配置

**测试用例：修改函数路由配置**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 修改路由项 | 调整 group, order, hidden 等 |
| 2 | 保存配置 | 调用 PUT /api/v1/functions/{id}/route |
| 3 | 刷新菜单 | 菜单结构按新配置渲染 |
| 4 | 持久化验证 | 重启后配置保持 |

**验证命令：**
```bash
# 更新路由配置
curl -X PUT http://localhost:8080/api/v1/functions/test.function.v1/route \
  -H "Content-Type: application/json" \
  -d '{
    "group": "Favorites",
    "order": 10,
    "hidden": false
  }'

# 预期响应：
# {
#   "success": true
# }
```

### 5.3 隐藏/显示函数

**测试用例：通过 hidden 控制函数可见性**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 设置 hidden=true | 函数从菜单消失 |
| 2 | 设置 hidden=false | 函数重新出现 |
| 3 | 直接访问 URL | 即使 hidden 也可直接访问 |

---

## 阶段六：函数调用

### 6.1 表单渲染

**测试用例：Formily 表单正确渲染**

| 检查项 | 验证点 |
|--------|--------|
| 文本输入 | Input 组件正确绑定 |
| 数字输入 | InputNumber 组件 |
| 选择器 | Select/Select 多选 |
| 日期选择 | DatePicker/RangePicker |
| 开关 | Switch 组件 |
| 必填校验 | required 字段显示星号 |
| 正则校验 | pattern 校验生效 |

### 6.2 参数校验

**测试用例：表单校验规则生效**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 提交空表单 | 必填字段显示错误 |
| 2 | 输入非法值 | 正则校验失败提示 |
| 3 | 输入超出范围 | min/max 校验生效 |
| 4 | 修正后提交 | 校验通过，发起调用 |

### 6.3 函数调用

**测试用例：通过 Dashboard 调用函数**

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 填写表单 | 输入符合校验的参数 |
| 2 | 点击调用 | POST /api/v1/functions/{id}/invoke |
| 3 | 调用成功 | 显示返回结果 |
| 4 | 日志记录 | 调用记录出现在审计日志 |

**验证命令：**
```bash
# 调用函数
curl -X POST http://localhost:8080/api/v1/functions/test.function.v1/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: test-game" \
  -d '{
    "player_id": "12345",
    "amount": 100
  }'

# 预期响应：
# {
#   "success": true,
#   "data": {
#     "result": "ok",
#     "new_balance": 1000
#   }
# }
```

---

## 完整链路验收

### 验收标准汇总

| 阶段 | 关键指标 | 验收标准 |
|------|----------|----------|
| SDK → Agent | 注册成功率 | 100% |
| Agent → Server | 同步延迟 | < 5s |
| Dashboard 展示 | 菜单更新延迟 | < 10s |
| UI 配置 | 保存响应时间 | < 1s |
| UI 历史 | 查询响应时间 | < 500ms |
| UI 回滚 | 回滚成功率 | 100% |
| 函数调用 | 端到端延迟 | < 2s |

### 性能基准测试

```bash
# 批量注册函数（100个）
for i in {1..100}; do
  # 注册函数
done

# 验证：
# - Server 能承受 100 函数并发注册
# - Dashboard 菜单渲染不卡顿
# - 函数搜索响应 < 200ms
```

---

## 回归测试清单

每次更新后执行：

```bash
# 1. 服务健康检查
make test

# 2. API 端点测试
curl http://localhost:8080/api/v1/health
curl http://localhost:8080/api/v1/functions/descriptors

# 3. 前端类型检查
cd croupier-dashboard && pnpm tsc

# 4. 端到端关键流程
# - 登录 → 查看函数列表 → 点击详情 → 编辑 UI 配置 → 保存 → 查看历史 → 回滚
```

---

## 常见问题排查

### 问题：函数未出现在菜单

1. 检查 Agent 是否连接成功
2. 检查函数是否已同步到 Server
3. 检查 hidden 配置是否为 true
4. 检查 category/group 路由配置

### 问题：UI 配置保存失败

1. 检查请求体格式是否正确
2. 检查函数 ID 是否存在
3. 查看 Server 日志确认错误原因

### 问题：历史记录为空

1. 确认至少进行过一次 UI 配置更新
2. 检查数据库连接是否正常
3. 查看后台 UI 版本服务日志
