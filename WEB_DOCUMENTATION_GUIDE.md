# Croupier Web 前端文档指南

Croupier Web 前端已生成完整的技术文档。所有文档位于 `web/` 目录。

## 快速导航

### 从这里开始
- **[web/WEB_DOCUMENTATION_INDEX.md](web/WEB_DOCUMENTATION_INDEX.md)** - 文档导航索引和快速导航表

### 常用文档
- **[web/QUICK_REFERENCE.md](web/QUICK_REFERENCE.md)** - 日常开发速查表(最常用)
- **[web/FRONTEND_ANALYSIS.md](web/FRONTEND_ANALYSIS.md)** - 完整的架构分析

### 实践指南
- **[web/CONFIGURATION_EXAMPLE.md](web/CONFIGURATION_EXAMPLE.md)** - 配置代码示例
- **[web/EXAMPLE_USERS_PAGE.tsx](web/EXAMPLE_USERS_PAGE.tsx)** - 页面组件模板

## 不同用户的推荐阅读

### 新员工(30分钟)
1. [web/WEB_DOCUMENTATION_INDEX.md](web/WEB_DOCUMENTATION_INDEX.md)
2. [web/QUICK_REFERENCE.md](web/QUICK_REFERENCE.md)
3. 启动开发环境: `cd web && pnpm dev`
4. [web/CONFIGURATION_EXAMPLE.md](web/CONFIGURATION_EXAMPLE.md) + [web/EXAMPLE_USERS_PAGE.tsx](web/EXAMPLE_USERS_PAGE.tsx)

### 想添加新菜单项(20分钟)
1. [web/QUICK_REFERENCE.md](web/QUICK_REFERENCE.md) - 看"文件快速定位"部分
2. [web/CONFIGURATION_EXAMPLE.md](web/CONFIGURATION_EXAMPLE.md) - 复制代码
3. [web/EXAMPLE_USERS_PAGE.tsx](web/EXAMPLE_USERS_PAGE.tsx) - 参考实现

### 想深入理解架构(1小时)
1. [web/FRONTEND_ANALYSIS.md](web/FRONTEND_ANALYSIS.md) - 完整阅读
2. 对照现有代码学习
3. 参考文档实现新功能

### 遇到问题(5分钟)
- [web/QUICK_REFERENCE.md](web/QUICK_REFERENCE.md) - "常见错误排查"表

## 文档内容简述

| 文档 | 行数 | 大小 | 用途 |
|------|------|------|------|
| WEB_DOCUMENTATION_INDEX | 283 | 7.5K | 导航和 FAQ |
| FRONTEND_ANALYSIS | 598 | 16K | 完整分析 |
| QUICK_REFERENCE | 237 | 6.2K | 日常参考 |
| CONFIGURATION_EXAMPLE | 178 | 4.7K | 代码示例 |
| EXAMPLE_USERS_PAGE | 105 | 2.6K | 页面模板 |

## 技术栈速览

- **框架**: Umi Max 4 + React 18
- **UI**: Ant Design 5 + Pro 2.7
- **权限**: RBAC (Umi Max 原生)
- **API**: umijs/max request
- **i18n**: 8 种语言

## 关键命令

```bash
# 开发
cd web && pnpm dev         # 启动前端(端口8000)
go run ./cmd/server        # 启动后端(端口8080)

# 代码质量
pnpm lint                  # 代码检查
pnpm build                 # 生产构建

# 登录凭证(开发)
username: admin
password: admin123
```

## 快速问答

**Q: 怎样添加新菜单项?**  
A: 修改 5 个文件(routes.ts, access.ts, 两个 menu.ts, 页面组件)。看 CONFIGURATION_EXAMPLE.md

**Q: 权限系统怎样工作的?**  
A: RBAC(resource:action 格式)。三层检查:路由级(隐藏菜单) + 按钮级(禁用) + 功能级(提示)

**Q: 菜单不显示怎么办?**  
A: 检查 routes.ts 配置、菜单翻译、权限检查。看 QUICK_REFERENCE.md 的错误排查表

**Q: 如何调试 API?**  
A: 浏览器 Network 标签查看请求。确保后端在 http://localhost:8080

更多常见问题，见 [WEB_DOCUMENTATION_INDEX.md](web/WEB_DOCUMENTATION_INDEX.md)

---

所有文档已保存在 `web/` 目录。祝开发愉快！
