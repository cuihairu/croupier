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

---

## 自动化测试脚本

### 一键健康检查脚本

```bash
#!/bin/bash
# scripts/e2e/health-check.sh

set -euo pipefail

# 配置
SERVER_URL="${SERVER_URL:-http://localhost:8080}"
DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:8000}"
GAME_ID="${GAME_ID:-test-game}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试结果统计
PASSED=0
FAILED=0

# 测试函数
test_service() {
    local name="$1"
    local url="$2"
    local expected_code="${3:-200}"

    echo -n "Testing $name... "
    code=$(curl -s -o /dev/null -w "%{http_code}" "$url")

    if [ "$code" = "$expected_code" ]; then
        echo -e "${GREEN}PASS${NC} ($code)"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}FAIL${NC} (expected $expected_code, got $code)"
        ((FAILED++))
        return 1
    fi
}

echo "=== Croupier E2E Health Check ==="
echo "Server: $SERVER_URL"
echo "Dashboard: $DASHBOARD_URL"
echo "Game ID: $GAME_ID"
echo ""

# 服务健康检查
test_service "Server Health" "$SERVER_URL/healthz"
test_service "Agent Status" "$SERVER_URL/api/v1/agents"
test_service "Function Descriptors" "$SERVER_URL/api/v1/functions/descriptors"
test_service "Dashboard" "$DASHBOARD_URL"

# API 端点检查
test_service "Function List" "$SERVER_URL/api/v1/functions"
test_service "Games List" "$SERVER_URL/api/v1/games"

echo ""
echo "=== Results ==="
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}Some tests failed!${NC}"
    exit 1
fi
```

### 函数注册链路测试脚本

```bash
#!/bin/bash
# scripts/e2e/test-function-registration.sh

set -euo pipefail

SERVER_URL="${SERVER_URL:-http://localhost:8080}"
FUNCTION_ID="test.e2e.function.$(date +%s)"

echo "=== Testing Function Registration Flow ==="
echo "Function ID: $FUNCTION_ID"

# 1. 检查函数是否已注册
echo ""
echo "Step 1: Check if function exists in registry"
code=$(curl -s -o /dev/null -w "%{http_code}" \
    "$SERVER_URL/api/v1/functions/$FUNCTION_ID")
echo "HTTP Status: $code"

# 2. 获取函数 OpenAPI
echo ""
echo "Step 2: Get function OpenAPI spec"
openapi=$(curl -s "$SERVER_URL/api/v1/functions/$FUNCTION_ID/openapi")
echo "$openapi" | jq .

# 3. 获取函数描述
echo ""
echo "Step 3: Get function descriptor"
descriptor=$(curl -s "$SERVER_URL/api/v1/functions/descriptors" \
    | jq ".descriptors[] | select(.id == \"$FUNCTION_ID\")")
echo "$descriptor"

# 4. 测试 UI 配置读取
echo ""
echo "Step 4: Get UI configuration"
ui_config=$(curl -s "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui")
echo "$ui_config" | jq .

# 5. 测试路由配置读取
echo ""
echo "Step 5: Get route configuration"
route_config=$(curl -s "$SERVER_URL/api/v1/functions/$FUNCTION_ID/route")
echo "$route_config" | jq .

echo ""
echo "=== Registration Flow Test Complete ==="
```

### UI 配置测试脚本

```bash
#!/bin/bash
# scripts/e2e/test-ui-configuration.sh

set -euo pipefail

SERVER_URL="${SERVER_URL:-http://localhost:8080}"
FUNCTION_ID="${FUNCTION_ID:-test.e2e.function}"

echo "=== Testing UI Configuration Flow ==="

# 1. 读取当前 UI 配置
echo ""
echo "Step 1: Read current UI config"
current_ui=$(curl -s "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui")
echo "$current_ui" | jq .
current_version=$(echo "$current_ui" | jq -r '.version')

# 2. 更新 UI 配置
echo ""
echo "Step 2: Update UI configuration"
new_ui='{
  "layout": "horizontal",
  "labelWidth": 160,
  "size": "large"
}'
update_result=$(curl -s -X PUT "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui" \
    -H "Content-Type: application/json" \
    -d "$new_ui")
echo "$update_result" | jq .
new_version=$(echo "$update_result" | jq -r '.version')

# 3. 验证配置已更新
echo ""
echo "Step 3: Verify configuration updated"
updated_ui=$(curl -s "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui")
echo "$updated_ui" | jq .

# 4. 查看历史记录
echo ""
echo "Step 4: View UI history"
history=$(curl -s "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui/history")
echo "$history" | jq .

# 5. 回滚到之前版本
if [ -n "$current_version" ] && [ "$current_version" != "null" ]; then
    echo ""
    echo "Step 5: Rollback to version $current_version"
    rollback_result=$(curl -s -X POST \
        "$SERVER_URL/api/v1/functions/$FUNCTION_ID/ui/rollback" \
        -H "Content-Type: application/json" \
        -d "{\"version\": $current_version}")
    echo "$rollback_result" | jq .
fi

echo ""
echo "=== UI Configuration Test Complete ==="
```

---

## 测试数据模板

### 标准测试函数

```json
{
  "id": "test.player.addCurrency",
  "name": "添加玩家货币",
  "category": "player",
  "description": "给指定玩家添加指定数量的游戏货币",
  "tags": ["player", "currency", "test"],
  "parameters": {
    "type": "object",
    "properties": {
      "player_id": {
        "type": "string",
        "title": "玩家ID",
        "description": "玩家的唯一标识符",
        "minLength": 1,
        "maxLength": 64
      },
      "currency_type": {
        "type": "string",
        "title": "货币类型",
        "description": "货币类型：gold, diamond, coupon",
        "enum": ["gold", "diamond", "coupon"],
        "default": "gold"
      },
      "amount": {
        "type": "integer",
        "title": "数量",
        "description": "添加的货币数量",
        "minimum": 1,
        "maximum": 1000000,
        "default": 100
      },
      "reason": {
        "type": "string",
        "title": "原因",
        "description": "添加原因",
        "maxLength": 256
      }
    },
    "required": ["player_id", "amount"]
  },
  "response": {
    "type": "object",
    "properties": {
      "success": {"type": "boolean"},
      "new_balance": {"type": "integer"},
      "previous_balance": {"type": "integer"}
    }
  }
}
```

### UI 配置模板

```json
{
  "layout": "vertical",
  "labelWidth": 120,
  "size": "middle",
  "colon": true,
  "feedbackLayout": "loose",
  "wrapperWidth": "100%",
  "components": {
    "player_id": {
      "x-component": "Input",
      "x-component-props": {
        "placeholder": "请输入玩家ID",
        "allowClear": true
      },
      "x-decorator": "Required"
    },
    "currency_type": {
      "x-component": "Select",
      "x-component-props": {
        "placeholder": "选择货币类型",
        "allowClear": true
      }
    },
    "amount": {
      "x-component": "InputNumber",
      "x-component-props": {
        "min": 1,
        "max": 1000000,
        "precision": 0,
        "style": { "width": "100%" }
      },
      "x-decorator": "Required"
    },
    "reason": {
      "x-component": "TextArea",
      "x-component-props": {
        "placeholder": "请输入添加原因",
        "rows": 3,
        "maxLength": 256,
        "showCount": true
      }
    }
  }
}
```

---

## 边界测试用例

### 空值与异常输入测试

| 测试场景 | 输入 | 预期结果 |
|----------|------|----------|
| 空 player_id | `player_id: ""` | 校验失败，提示必填 |
| 超长 player_id | 65 字符字符串 | 校验失败，提示超长 |
| 负数金额 | `amount: -100` | 校验失败，提示最小值 |
| 零金额 | `amount: 0` | 校验失败，提示最小值 |
| 超大金额 | `amount: 1000001` | 校验失败，提示最大值 |
| 无效枚举值 | `currency_type: "invalid"` | 校验失败，提示无效值 |
| 特殊字符注入 | `player_id: "<script>"` | 正常处理或转义 |

### 并发测试

```bash
#!/bin/bash
# scripts/e2e/test-concurrent.sh

# 并发调用测试 - 模拟多个管理员同时操作
FUNCTION_ID="${FUNCTION_ID:-test.player.addCurrency}"
CONCURRENT=10
SERVER_URL="${SERVER_URL:-http://localhost:8080}"

echo "=== Concurrent Function Invocation Test ==="
echo "Concurrent requests: $CONCURRENT"

for i in $(seq 1 $CONCURRENT); do
    (
        curl -s -X POST "$SERVER_URL/api/v1/functions/$FUNCTION_ID/invoke" \
            -H "Content-Type: application/json" \
            -H "X-Game-ID: test-game" \
            -d "{\"player_id\": \"player_$i\", \"amount\": 100}" \
            -o "/tmp/result_$i.json" \
            -w "%{http_code}\n"
    ) &
done

wait

echo ""
echo "=== Results ==="
success=0
for i in $(seq 1 $CONCURRENT); do
    if [ -f "/tmp/result_$i.json" ]; then
        code=$(head -1 "/tmp/result_$i.json")
        if [ "$code" = "200" ]; then
            ((success++))
        fi
        rm -f "/tmp/result_$i.json"
    fi
done

echo "Successful: $success/$CONCURRENT"
```

---

## 性能测试脚本

### API 响应时间测试

```bash
#!/bin/bash
# scripts/e2e/test-performance.sh

set -euo pipefail

SERVER_URL="${SERVER_URL:-http://localhost:8080}"

# 测试函数列表响应时间
echo "=== Performance Test: API Response Times ==="

test_endpoint() {
    local name="$1"
    local url="$2"

    echo -n "$name: "

    # 执行 10 次取平均
    total=0
    for i in $(seq 1 10); do
        time=$(curl -s -o /dev/null -w "%{time_total}" "$url")
        total=$(echo "$total + $time" | bc)
    done

    avg=$(echo "scale=3; $total / 10" | bc)

    # 判断性能
    if (( $(echo "$avg < 0.1" | bc -l) )); then
        echo -e "${GREEN}$avg s${NC} (Excellent)"
    elif (( $(echo "$avg < 0.5" | bc -l) )); then
        echo -e "${GREEN}$avg s${NC} (Good)"
    elif (( $(echo "$avg < 1.0" | bc -l) )); then
        echo -e "${YELLOW}$avg s${NC} (Acceptable)"
    else
        echo -e "${RED}$avg s${NC} (Slow)"
    fi
}

test_endpoint "Health Check" "$SERVER_URL/healthz"
test_endpoint "Function Descriptors" "$SERVER_URL/api/v1/functions/descriptors"
test_endpoint "Function List" "$SERVER_URL/api/v1/functions"
test_endpoint "Agents Status" "$SERVER_URL/api/v1/agents"
test_endpoint "Games List" "$SERVER_URL/api/v1/games"

echo ""
echo "=== Performance Test Complete ==="
```

### Dashboard 加载性能测试

```bash
#!/bin/bash
# scripts/e2e/test-dashboard-performance.sh

# 使用 Puppeteer/Playwright 进行前端性能测试
# 需要先安装：npm install -g @playwright/test

cat > /tmp/dashboard-perf.spec.js << 'EOF'
const { test, expect } = require('@playwright/test');

test('Dashboard load performance', async ({ page }) => {
  const startTime = Date.now();

  await page.goto('http://localhost:8000');
  await page.waitForLoadState('networkidle');

  const loadTime = Date.now() - startTime;
  console.log(`Page load time: ${loadTime}ms`);

  expect(loadTime).toBeLessThan(3000);
});

test('Function menu render performance', async ({ page }) => {
  await page.goto('http://localhost:8000');
  await page.waitForSelector('.ant-menu');

  const startTime = Date.now();
  await page.click('text=Game');
  await page.waitForSelector('text=Registered');

  const renderTime = Date.now() - startTime;
  console.log(`Menu render time: ${renderTime}ms`);

  expect(renderTime).toBeLessThan(500);
});
EOF

# 运行测试
playwright test /tmp/dashboard-perf.spec.js
```

---

## 测试报告模板

### 验收测试报告

```markdown
# 函数注册到 Dashboard 展示 - 验收测试报告

**测试日期**: YYYY-MM-DD
**测试人员**: [姓名]
**测试环境**: [环境信息]

---

## 1. 测试概览

| 测试项 | 用例数 | 通过 | 失败 | 跳过 |
|--------|--------|------|------|------|
| SDK → Agent 注册 | 5 | 5 | 0 | 0 |
| Agent → Server 同步 | 8 | 8 | 0 | 0 |
| Dashboard 展示 | 10 | 9 | 1 | 0 |
| UI 配置编辑 | 12 | 12 | 0 | 0 |
| 菜单路由编辑 | 6 | 6 | 0 | 0 |
| 函数调用 | 7 | 7 | 0 | 0 |
| **合计** | **48** | **47** | **1** | **0** |

**通过率**: 97.9%

---

## 2. 失败用例详情

| 用例 ID | 用例名称 | 失败原因 | 严重程度 | 状态 |
|---------|----------|----------|----------|------|
| TC-DASH-004 | 函数详情页加载 | 响应超时 > 3s | 中 | 待修复 |

---

## 3. 性能测试结果

| API 端点 | 平均响应时间 | P95 响应时间 | 状态 |
|----------|-------------|-------------|------|
| /healthz | 15ms | 20ms | ✅ |
| /functions/descriptors | 120ms | 180ms | ✅ |
| /functions | 80ms | 150ms | ✅ |
| /functions/{id}/ui | 45ms | 80ms | ✅ |

---

## 4. 问题汇总

| 问题 ID | 描述 | 优先级 | 责任人 | 状态 |
|---------|------|--------|--------|------|
| BUG-001 | 函数详情页偶尔加载超时 | 高 | [姓名] | 待修复 |

---

## 5. 验收结论

- [ ] 所有关键用例通过
- [ ] 性能指标达标
- [ ] 无阻塞性问题

**验收结论**: [通过 / 不通过 / 附带条件通过]

**签字**: ____________    **日期**: ____________
```

---

## 快速验收命令

### 完整验收（一键运行）

```bash
#!/bin/bash
# scripts/e2e/full-acceptance-test.sh

set -euo pipefail

echo "=========================================="
echo "  Croupier E2E Acceptance Test Suite"
echo "=========================================="
echo ""

# 阶段 1: 环境检查
echo ">>> Stage 1: Environment Check"
./scripts/e2e/health-check.sh || exit 1

# 阶段 2: 函数注册测试
echo ""
echo ">>> Stage 2: Function Registration"
./scripts/e2e/test-function-registration.sh || exit 1

# 阶段 3: UI 配置测试
echo ""
echo ">>> Stage 3: UI Configuration"
./scripts/e2e/test-ui-configuration.sh || exit 1

# 阶段 4: 性能测试
echo ""
echo ">>> Stage 4: Performance"
./scripts/e2e/test-performance.sh || exit 1

# 阶段 5: 并发测试
echo ""
echo ">>> Stage 5: Concurrent"
./scripts/e2e/test-concurrent.sh || exit 1

echo ""
echo "=========================================="
echo "  All Acceptance Tests PASSED!"
echo "=========================================="
```
