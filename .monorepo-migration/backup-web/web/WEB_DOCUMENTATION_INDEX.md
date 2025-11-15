# Croupier Web 前端 - 文档索引

本目录包含 Croupier Web 前端项目的完整技术文档。请根据需要阅读相应的文档。

## 文档清单

### 1. FRONTEND_ANALYSIS.md (16 KB)
**完整的前端架构分析报告**

内容包括:
- 技术栈详细说明(Umi Max, React 18, Ant Design)
- 项目结构和文件映射
- 路由和菜单系统实现原理
- 权限系统(RBAC)完整说明
- 现有页面分析(Assignments, Audit, Approvals, Registry)
- 如何添加新权限管理页面的 5 步指南
- 权限命名约定和最佳实践
- 国际化(i18n)系统说明
- 现存问题和优化建议

**推荐人群**: 架构师、技术负责人、想深入理解系统设计的开发者

**阅读时间**: 30-40 分钟

---

### 2. QUICK_REFERENCE.md (6.2 KB)
**快速参考卡片 - 日常开发速查表**

内容包括:
- 项目架构一览表
- 关键文件映射(5 个文件添加新菜单)
- 权限系统速查表
- 常见开发任务(4 个)
- API 服务层速查
- 现有权限相关页面总结
- 开发环境启动命令
- 常见错误排查表

**推荐人群**: 日常开发者、新员工入职

**阅读时间**: 5-10 分钟

**使用场景**: 
- "我要添加一个新菜单项" → 看这个文档
- "我要修改权限" → 看第 2 部分
- "API 文档在哪" → 看第 5 部分

---

### 3. CONFIGURATION_EXAMPLE.md (4.7 KB)
**配置示例代码**

内容包括:
- 完整的配置示例(5 个文件)
- 添加"用户管理"和"角色管理"两个菜单项的真实代码
- 路由配置示例
- 权限定义示例
- 菜单翻译示例
- API 函数定义示例

**推荐人群**: 实际开发者、想看具体代码的人

**阅读时间**: 5-10 分钟

**使用场景**: 
- 复制粘贴代码时参考这个文档

---

### 4. EXAMPLE_USERS_PAGE.tsx (2.6 KB)
**用户管理页面完整示例**

内容包括:
- 完整的 React 组件实现
- 权限检查逻辑
- 表格、搜索、分页
- 创建/编辑/删除用户
- 模态框和抽屉组件使用

**推荐人群**: 实际开发者、想了解组件结构的人

**阅读时间**: 10 分钟

**使用场景**:
- 新建页面时参考这个模板

---

### 5. README.md (1.3 KB)
**项目快速启动指南**

原始文档,包含:
- 项目简介
- 快速启动步骤
- 开发脚本
- 后端 API 期望
- 默认登录凭证

---

## 快速导航

### 我想...

| 需求 | 推荐文档 | 部分 |
|------|---------|------|
| 了解整个前端架构 | FRONTEND_ANALYSIS | 1-5 |
| 快速添加新菜单项 | QUICK_REFERENCE | "文件快速定位" |
| 查看 API 调用方式 | QUICK_REFERENCE | "API 服务层速查" |
| 理解权限系统 | FRONTEND_ANALYSIS | 4, 7 |
| 启动开发环境 | QUICK_REFERENCE | "开发环境启动" |
| 看具体代码示例 | CONFIGURATION_EXAMPLE | 完整文件 |
| 创建新页面组件 | EXAMPLE_USERS_PAGE | 完整文件 |
| 排查错误 | QUICK_REFERENCE | "常见错误排查" |

---

## 关键概念速览

### 技术栈
- **前端框架**: Umi Max 4 + React 18.2
- **UI 组件库**: Ant Design 5.20 + Pro 2.7
- **权限系统**: RBAC (基于 Umi Max 内置)
- **API 调用**: umijs/max request
- **国际化**: Umi i18n (8 种语言)

### 核心文件
```
config/routes.ts              路由 + 菜单 + 权限定义
src/access.ts                 权限检查函数
src/pages/*/index.tsx         页面组件
src/services/croupier/        API 调用层
src/locales/*/menu.ts         菜单国际化文本
```

### 权限命名规则
```
<resource>:<action>
examples: users:read, users:write, roles:read, audit:read
```

### 添加新菜单的 5 个步骤
1. `config/routes.ts` - 定义路由
2. `src/access.ts` - 定义权限
3. `src/locales/en-US/menu.ts` - 英文菜单
4. `src/locales/zh-CN/menu.ts` - 中文菜单
5. `src/pages/YourPage/index.tsx` - 创建页面

---

## 项目结构概览

```
web/
├── config/
│   ├── routes.ts             路由配置(必改)
│   ├── defaultSettings.ts     布局设置
│   └── proxy.ts              开发代理
├── src/
│   ├── pages/                页面组件(新建页面放这里)
│   ├── components/           共享组件
│   ├── services/croupier/    API 调用(必改)
│   ├── locales/              国际化文本(必改)
│   │   ├── en-US/menu.ts
│   │   └── zh-CN/menu.ts
│   ├── access.ts             权限定义(必改)
│   ├── app.tsx               App 入口
│   └── global.tsx            全局样式
├── public/                   静态资源
└── package.json
```

---

## 常见开发任务清单

### 添加新菜单页面
- [ ] 读 QUICK_REFERENCE "文件快速定位" 部分
- [ ] 复制 CONFIGURATION_EXAMPLE 中的代码到 5 个文件
- [ ] 创建 src/pages/YourPage/index.tsx
- [ ] 参考 EXAMPLE_USERS_PAGE.tsx 实现页面逻辑
- [ ] 重启开发服务器: pnpm dev
- [ ] 浏览器访问 http://localhost:8000 测试

### 添加权限检查
- [ ] 在 src/access.ts 中定义新的权限函数
- [ ] 在路由中使用 access 属性(隐藏菜单)
- [ ] 在页面中用权限函数控制按钮(禁用/隐藏)
- [ ] 测试没有权限的用户看不到菜单/按钮

### 调用后端 API
- [ ] 在 src/services/croupier/index.ts 中定义 API 函数
- [ ] 在页面中 import 该函数
- [ ] 使用 try-catch 处理错误
- [ ] 用 message.success/error 提示用户

---

## 开发环境启动

```bash
# 1. 安装依赖
cd web && pnpm install

# 2. 启动前端开发服务器 (端口 8000)
pnpm dev

# 3. 在另一个终端启动后端 (端口 8080)
cd .. && go run ./cmd/server -c configs/server.example.yaml

# 4. 浏览器访问 http://localhost:8000
# 登录: admin / admin123
```

---

## 现有权限管理页面

| 页面 | 路径 | 权限 | 核心功能 |
|------|------|------|---------|
| Assignments | /gm/assignments | assignments:read/write | 配置允许调用的函数 |
| Audit | /gm/audit | audit:read | 查看操作日志, 导出 CSV |
| Approvals | /gm/approvals | 无 | 批准/拒绝函数调用 |
| Registry | /gm/registry | registry:read | 查看代理和函数覆盖率 |

---

## 推荐阅读顺序

### 对于新员工
1. README.md (5 分钟) - 了解项目
2. QUICK_REFERENCE.md (10 分钟) - 学习基础概念
3. 启动开发环境 (5 分钟)
4. CONFIGURATION_EXAMPLE.md (10 分钟) - 学习如何配置
5. EXAMPLE_USERS_PAGE.tsx (10 分钟) - 学习代码结构

### 对于项目维护者
1. FRONTEND_ANALYSIS.md (40 分钟) - 深入理解架构
2. QUICK_REFERENCE.md (5 分钟) - 日常参考
3. CONFIGURATION_EXAMPLE.md (10 分钟) - 标准化配置

---

## 常见问题

### Q: 菜单不显示怎么办?
A: 检查 config/routes.ts 中 path, name, component 是否正确, 以及相应的菜单翻译是否存在

### Q: 权限总是检查失败?
A: 确保在 src/access.ts 中定义了对应的权限函数, 并且用户实际拥有该权限

### Q: 怎样调试后端 API?
A: 在浏览器开发者工具 Network 标签中查看请求和响应, 确保后端服务在 http://localhost:8080

### Q: 如何支持新语言?
A: 在 src/locales/ 目录下创建新的语言文件夹(如 es-ES), 然后添加翻译内容

---

## 有用的链接

- Umi Max: https://umijs.org/docs/max
- Ant Design: https://ant.design/
- Ant Design Pro: https://procomponents.ant.design/
- TypeScript: https://www.typescriptlang.org/

---

## 文档更新历史

- 2025-11-02: 初版文档创建
  - 添加 FRONTEND_ANALYSIS.md (详细架构分析)
  - 添加 QUICK_REFERENCE.md (快速参考)
  - 添加 CONFIGURATION_EXAMPLE.md (配置示例)
  - 添加 EXAMPLE_USERS_PAGE.tsx (完整示例)

---

## 反馈和改进

如有任何问题或改进建议, 欢迎提交 Issue 或 Pull Request!

