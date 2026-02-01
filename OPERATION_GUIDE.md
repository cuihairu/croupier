# 🎯 Croupier 系统操作指南

## 📋 目录
1. [启动服务](#启动服务)
2. [访问前端](#访问前端)
3. [导入函数 Pack](#导入函数-pack)
4. [查看和使用函数](#查看和使用函数)
5. [配置 UI、路由、权限](#配置-ui路由权限)
6. [常见问题](#常见问题)

---

## 🚀 启动服务

### 后端服务（Go）

```bash
# 进入后端目录
cd /Users/cui/Workspaces/croupier/croupier/services/server

# 编译并启动
make build
./bin/server -f configs/server.yaml

# 或者直接运行
go run server.go -f configs/server.yaml
```

**后端地址**:
- HTTP API: `http://localhost:8080`
- gRPC: `localhost:8443`

### 前端服务（React）

```bash
# 进入前端目录
cd /Users/cui/Workspaces/croupier/croupier-dashboard

# 安装依赖（首次运行）
pnpm install

# 启动开发服务器
pnpm dev
```

**前端地址**: `http://localhost:8000`

---

## 🌐 访问前端

### 1. 登录系统

打开浏览器访问: `http://localhost:8000`

默认账号密码（查看配置文件）:
```yaml
# configs/server.yaml 或 configs/users.yaml
```

### 2. 主要页面路径

| 页面 | 路径 | 功能 |
|------|------|------|
| **函数目录** | `/game/functions/catalog` | 查看所有已注册函数 |
| **函数实例** | `/game/functions/instances` | 查看函数运行实例 |
| **函数 Packs** | `/game/functions/packs` | **管理函数包（上传、导入）** ⭐ |
| **组件管理** | `/components` 或 `/game/component-management` | 组件管理中心 |
| **权限管理** | `/admin/permissions` | 配置函数权限 |

---

## 📦 导入函数 Pack

### **方法一：通过前端界面导入（推荐）** ⭐

1. **打开 Pack 管理页面**:
   - 访问: `http://localhost:8000/game/functions/packs`
   - 或在侧边栏菜单点击: **Game Management** → **Function Packs**

2. **上传 Pack 文件**:
   - 点击页面右上角 **"上传 Pack"** 按钮
   - 选择准备好的 `.tgz` 文件（如 `player.tgz`）
   - 选择游戏环境（Game ID）
   - 点击 **确认上传**

3. **查看导入结果**:
   - 成功后会在列表中看到新的 Pack
   - 显示函数数量、描述符数量、UI Schema 数量

### **方法二：通过 API 导入**

```bash
# 1. 先打包（如果还没有）
cd /Users/cui/Workspaces/croupier/croupier/packs/player
./pack.sh

# 2. 通过 curl 上传
curl -X POST http://localhost:8080/api/v1/packs/import \
  -F "pack=@player.tgz" \
  -H "X-Game-ID: your-game-id"

# 返回示例
{
  "code": 0,
  "message": "Pack imported successfully",
  "data": {
    "pack_id": "player",
    "functions_count": 1,
    "descriptors_count": 1,
    "ui_schema_count": 1
  }
}
```

### **准备示例 Pack（如果还没有）**

```bash
# 1. 进入示例目录
cd /Users/cui/Workspaces/croupier/croupier/packs/player

# 2. 查看文件
ls -l
# -rw-r--r-- descriptors/player.get.json
# -rw-r--r-- ui/player.get.uischema.json
# -rwxr-xr-x pack.sh

# 3. 打包
./pack.sh

# 4. 查看生成的文件
ls -lh player.tgz
```

---

## 👀 查看和使用函数

### **步骤 1：查看函数目录**

1. 访问: `http://localhost:8000/game/functions/catalog`
2. 可以看到所有已导入的函数列表
3. 列表显示:
   - 函数ID（如 `player.get`）
   - 函数名称（如 "获取玩家信息"）
   - 分类（如 `player`）
   - 状态（启用/禁用）
   - 版本号

### **步骤 2：调用函数**

**方法 A：从函数目录调用** ⭐

1. 在函数列表中找到要调用的函数
2. 点击操作列的 **▶️ 调用函数** 按钮
3. 自动跳转到函数工作台（如 `/game/player/get?fid=player.get`）
4. 在工作台中填写参数并调用

**方法 B：直接访问工作台**

直接在浏览器输入路径（如果配置了 `menu.path`）:
```
http://localhost:8000/game/player/get?fid=player.get
```

### **步骤 3：在函数工作台中操作**

函数工作台 (`/game/functions?fid=xxx`) 提供:

1. **参数填写区**:
   - 根据 JSON Schema 自动生成表单
   - 根据 UI Schema 渲染组件
   - 支持字段验证

2. **调用操作**:
   - 选择游戏环境（Game ID）
   - 选择实例（如果有多个）
   - 点击 **"调用函数"** 按钮

3. **结果展示**:
   - JSON 视图
   - 表格视图
   - 图表视图（如果配置了）

---

## ⚙️ 配置 UI、路由、权限

### **流程概述**

```
创建 Pack (descriptor + ui schema)
    ↓
打包成 .tgz
    ↓
导入到系统
    ↓
自动注册函数 + UI + 路由
```

### **1. 配置 UI（UI Schema）**

**文件位置**: `packs/player/ui/player.get.uischema.json`

**示例配置**:
```json
{
  "ui:layout": {
    "type": "grid",      // grid | tabs
    "cols": 2            // 列数
  },
  "ui:groups": [
    {
      "title": "基础信息",
      "fields": ["player_id"]
    }
  ],
  "ui:fields": {
    "player_id": {
      "widget": "input",  // input | textarea | select | switch | date
      "placeholder": "请输入玩家ID",
      "rules": [
        { "required": true, "message": "不能为空" }
      ]
    }
  }
}
```

**修改后重新导入**:
```bash
cd /Users/cui/Workspaces/croupier/croupier/packs/player
./pack.sh
# 然后通过前端或 API 重新上传
```

### **2. 配置路由（menu 字段）**

**文件位置**: `packs/player/descriptors/player.get.json` 中的 `menu` 字段

**示例配置**:
```json
{
  "id": "player.get",
  "menu": {
    "section": "玩家管理",        // 一级菜单
    "group": "基础功能",          // 二级分组
    "path": "/game/player/get",  // 自定义路由路径 ⭐
    "order": 10,                 // 显示顺序
    "hidden": false              // 是否在菜单隐藏
  }
}
```

**路由使用**:
- 点击"调用函数"按钮会跳转到 `menu.path` 路径
- 可以直接在浏览器访问该路径
- 前端路由自动生成

### **3. 配置权限（auth 字段）**

**文件位置**: `packs/player/descriptors/player.get.json` 中的 `auth` 字段

**示例配置**:
```json
{
  "auth": {
    "permission": "player.get",      // 权限标识
    "roles": ["admin", "gm", "operator"]  // 允许的角色
  }
}
```

**通过 API 动态修改**:
```bash
# 查看权限
curl http://localhost:8080/api/v1/functions/player.get/permissions

# 更新权限
curl -X PUT http://localhost:8080/api/v1/functions/player.get/permissions \
  -H "Content-Type: application/json" \
  -d '{
    "permissions": [
      {
        "resource": "player.get",
        "actions": ["read", "invoke"],
        "roles": ["admin", "gm", "operator"]
      }
    ]
  }'
```

**通过前端界面修改**:
1. 访问: `/admin/permissions`
2. 找到对应函数
3. 修改权限配置

---

## 🛠️ 常见问题

### Q1: 后端服务启动失败？

**检查端口占用**:
```bash
lsof -i :8080  # HTTP API 端口
lsof -i :8443  # gRPC 端口
```

**查看日志**:
```bash
# 后端日志通常在
tail -f /var/log/croupier/server.log
# 或者控制台输出
```

### Q2: 前端无法连接后端？

**检查代理配置**:
```bash
# 前端配置文件
cat /Users/cui/Workspaces/croupier/croupier-dashboard/.env

# 确认代理配置
# .env
# BASH_API_URL=http://localhost:8080
```

**测试后端 API**:
```bash
curl http://localhost:8080/healthz
```

### Q3: 导入 Pack 后看不到函数？

**检查步骤**:
1. 确认 Pack 上传成功（查看 Packs 列表）
2. 刷新函数目录页面
3. 检查函数是否被禁用
4. 查看浏览器控制台错误

**API 调试**:
```bash
# 查看已导入的 descriptors
curl http://localhost:8080/api/v1/functions/descriptors

# 查看特定函数
curl http://localhost:8080/api/v1/functions/player.get
```

### Q4: UI Schema 没有生效？

**检查 UI Schema**:
```bash
# 查看函数的 UI 配置
curl http://localhost:8080/api/v1/functions/player.get/ui
```

**确认 UI Schema 文件名**:
- 必须是: `{function_id}.uischema.json`
- 示例: `player.get.uischema.json`

**重新导入 Pack**:
```bash
cd /Users/cui/Workspaces/croupier/croupier/packs/player
./pack.sh
# 通过前端重新上传
```

### Q5: 如何修改已有函数的配置？

**方法 A：修改源文件重新导入**
```bash
# 1. 修改 descriptor 或 ui schema 文件
vim /Users/cui/Workspaces/croupier/croupier/packs/player/descriptors/player.get.json

# 2. 重新打包
cd /Users/cui/Workspaces/croupier/croupier/packs/player
./pack.sh

# 3. 重新上传（前端或 API）
```

**方法 B：通过 API 直接修改**
```bash
# 修改 UI 配置
curl -X PUT http://localhost:8080/api/v1/functions/player.get/ui \
  -H "Content-Type: application/json" \
  -d @ui_schema.json

# 修改权限
curl -X PUT http://localhost:8080/api/v1/functions/player.get/permissions \
  -H "Content-Type: application/json" \
  -d '{
    "permissions": [...]
  }'
```

### Q6: 如何调试函数调用？

**查看调用日志**:
```bash
# 后端日志
tail -f /var/log/croupier/server.log | grep "player.get"
```

**前端调试**:
1. 打开浏览器开发者工具（F12）
2. 切换到 Console 标签
3. 调用函数
4. 查看网络请求和返回结果

**使用 API 直接测试**:
```bash
curl -X POST http://localhost:8080/api/v1/functions/player.get/invoke \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: your-game-id" \
  -d '{
    "params": {
      "player_id": "123456"
    }
  }'
```

---

## 📊 管理界面快速访问

| 功能 | 访问路径 | 说明 |
|------|----------|------|
| **Pack 管理** | `/game/functions/packs` | 上传、查看、删除 Pack ⭐ |
| **函数目录** | `/game/functions/catalog` | 查看所有函数 |
| **函数工作台** | `/game/functions?fid=xxx` | 调用函数 ⭐ |
| **组件管理** | `/components` | 组件中心 |
| **权限管理** | `/admin/permissions` | 配置权限 |
| **操作日志** | `/operations/operation-logs` | 查看操作历史 |

---

## 🎯 推荐工作流程

### **首次配置**

1. ✅ 启动后端服务
2. ✅ 启动前端服务
3. ✅ 访问 `http://localhost:8000`
4. ✅ 登录系统
5. ✅ 打开 `/game/functions/packs`
6. ✅ 上传示例 `player.tgz`
7. ✅ 打开 `/game/functions/catalog`
8. ✅ 找到"获取玩家信息"函数
9. ✅ 点击"调用函数"按钮
10. ✅ 在工作台中测试调用

### **日常开发**

1. 修改 descriptor 或 ui schema 文件
2. 运行 `./pack.sh` 重新打包
3. 通过前端界面重新上传
4. 刷新函数目录查看效果
5. 测试调用

---

## 📚 更多文档

- **函数 Pack 配置**: `/Users/cui/Workspaces/croupier/croupier/packs/player/README.md`
- **系统架构**: `/Users/cui/Workspaces/croupier/croupier/CLAUDE.md`
- **API 文档**: http://localhost:8080/swagger (如果启用了)

---

**作者**: Croupier Team
**更新时间**: 2025-02-01
**版本**: v1.0.0
