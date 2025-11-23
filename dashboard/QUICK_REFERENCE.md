# Croupier Web 前端 - 快速参考卡片

## 项目架构一览表

| 层级 | 技术 | 文件位置 | 说明 |
|------|------|---------|------|
| **框架** | Umi Max 4 + React 18 | `config/` | 路由、配置、代理 |
| **UI 组件** | Ant Design 5 + Pro 2.7 | `src/pages/` | 页面模板 |
| **权限系统** | RBAC (Umi Max 内置) | `src/access.ts` | 路由级权限控制 |
| **API 调用** | umijs/max request | `src/services/croupier/` | 后端 HTTP 调用 |
| **状态管理** | Umi Max hooks | `src/pages/*/index.tsx` | useState + useModel |
| **国际化** | Umi i18n | `src/locales/` | 8 种语言支持 |
| **样式** | Less + antd-style | `src/global.less` | 全局样式 |

---

## 关键文件映射

### 添加新菜单页面的 5 个文件

1. **`config/routes.ts`** - 定义路由 + 权限
   ```typescript
   { path: '/gm/users', name: 'Users', access: 'canUsersRead', component: './Users' }
   ```

2. **`src/access.ts`** - 定义权限检查函数
   ```typescript
   canUsersRead: has('users:read'),
   canUsersWrite: has('users:write'),
   ```

3. **`src/locales/en-US/menu.ts`** - 英文菜单文本
   ```typescript
   'menu.GM.Users': 'Users',
   ```

4. **`src/locales/zh-CN/menu.ts`** - 中文菜单文本
   ```typescript
   'menu.GM.Users': '用户管理',
   ```

5. **`src/pages/Users/index.tsx`** - 页面实现组件
   ```typescript
   export default function UsersPage() { ... }
   ```

---

## 权限系统速查表

### 权限命名规则

```
resource:action
examples:
  audit:read          # 查看审计日志
  audit:write         # 修改审计(通常不需要)
  users:read          # 查看用户列表
  users:write         # 创建/编辑/删除用户
  roles:read          # 查看角色
  roles:write         # 创建/编辑/删除角色
  *                   # 超级权限(包含所有)
```

### 三层权限检查

```
1. 路由级别 (隐藏菜单)
   config/routes.ts: access: 'canUsersRead'

2. 按钮级别 (禁用按钮)
   <Button disabled={!canWrite}>Delete</Button>

3. 功能级别 (显示提示)
   if (!canRead) return <Card>No permission</Card>;
```

---

## 常见开发任务

### 任务 1: 添加新页面菜单项

```bash
# 1. 更新 5 个文件(如上)
# 2. 重启开发服务器
pnpm dev

# 3. 浏览器访问 http://localhost:8000
# 4. 菜单应出现在左侧导航栏
```

### 任务 2: 添加权限检查

```typescript
// 1. access.ts 中定义
canUsersDelete: has('users:delete'),

// 2. 路由中使用 (隐藏菜单)
access: 'canUsersRead'

// 3. 页面中使用 (禁用按钮)
const canDelete = canUsersDelete && !isSystemUser;
<Button disabled={!canDelete}>Delete</Button>
```

### 任务 3: 调用后端 API

```typescript
// 1. services/croupier/index.ts 中添加
export async function deleteUser(id: string) {
  return request<void>(`/api/users/${id}`, { method: 'DELETE' });
}

// 2. 页面中调用
import { deleteUser } from '@/services/croupier';
const onDelete = async (id: string) => {
  await deleteUser(id);
  message.success('Deleted');
  await load();
};
```

### 任务 4: 国际化多语言

```typescript
// 1. en-US/menu.ts
'menu.GM.Users': 'Users',

// 2. zh-CN/menu.ts
'menu.GM.Users': '用户管理',

// 3. 页面使用
import { FormattedMessage } from '@umijs/max';
<FormattedMessage id="menu.GM.Users" />
```

---

## API 服务层速查

### 现有 API 调用函数

```typescript
// Auth
loginAuth({ username, password })          // POST /api/auth/login
fetchMe()                                    // GET /api/users/current

// Functions
listDescriptors()                            // GET /api/descriptors
invokeFunction(id, payload, opts)           // POST /api/invoke
startJob(id, payload, opts)                 // POST /api/start_job
cancelJob(id)                               // POST /api/cancel_job
fetchJobResult(id)                          // GET /api/job_result

// Games
listGames()                                  // GET /api/games
addGame(game)                               // POST /api/games

// Audit
listAudit(params)                           // GET /api/audit

// Assignments
fetchAssignments(params)                    // GET /api/assignments
setAssignments(params)                      // POST /api/assignments

// Registry & Packs
fetchRegistry()                             // GET /api/registry
listPacks()                                 // GET /api/packs/list
reloadPacks()                               // POST /api/packs/reload
```

---

## 现有权限相关页面

| 页面 | 路径 | 权限 | 关键功能 |
|------|------|------|---------|
| **Assignments** | `/gm/assignments` | `assignments:read/write` | 配置允许调用的函数 |
| **Audit** | `/gm/audit` | `audit:read` | 查看操作日志、导出 CSV |
| **Approvals** | `/gm/approvals` | 无 | 批准/拒绝函数调用、OTP |
| **Registry** | `/gm/registry` | `registry:read` | 查看代理、函数、覆盖率 |

---

## 开发环境启动

```bash
# 1. 安装依赖
cd web && pnpm install

# 2. 启动前端开发服务器
pnpm dev
# http://localhost:8000

# 3. 在另一个终端启动后端
cd .. && go run ./cmd/server -c configs/server.example.yaml
# http://localhost:8080

# 4. 登录凭证
# 用户名: admin
# 密码: admin123
```

---

## 文件快速定位

```
添加菜单项 → config/routes.ts
改权限 → src/access.ts
改菜单文本 → src/locales/*/menu.ts
改 API → src/services/croupier/index.ts
改页面 → src/pages/*/index.tsx
```

---

## 常见错误排查

| 问题 | 原因 | 解决方案 |
|------|------|---------|
| 菜单不显示 | 未添加到 routes.ts | 检查 path、name、component 是否正确 |
| 菜单不可点击 | access 权限检查失败 | 确保用户有该权限,或移除 access 属性 |
| 菜单文本是代码 | i18n 未配置 | 在 locales/*/menu.ts 中添加相应的 key |
| API 404 | 后端端口错误 | 检查 config/proxy.ts 是否指向 http://localhost:8080 |
| 权限总是失败 | access.ts 中缺少对应的 has() 调用 | 在 access.ts 中定义所有新权限 |

---

## 推荐学习资源

- [Umi Max 官方文档](https://umijs.org/docs/max)
- [Ant Design 5 组件库](https://ant.design/components/overview-cn/)
- [Ant Design Pro](https://procomponents.ant.design/)
- [TypeScript 手册](https://www.typescriptlang.org/zh/docs/)
