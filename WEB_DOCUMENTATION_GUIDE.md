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

## 前端读取 Analytics 规范（JSON）
- 生成：`make analytics-spec`（调用 `scripts/export-analytics-spec.ps1`，读取 `configs/analytics/*.yaml` 并导出 JSON）
- 输出：`web/public/analytics-spec.json`
- 用法（Umi/React 示例）：
  ```ts
  // src/services/analyticsSpec.ts
  export async function loadAnalyticsSpec() {
    const res = await fetch('/analytics-spec.json');
    return res.json();
  }
  ```
- 展示：从 `metrics.metrics` 读取 `zh_name/zh_desc` 用于表格/筛选/Tooltip；从 `game_types.game_types` 读取 `summary/description`。

## 在后台“游戏管理”页面展示游戏类型信息
- 预置：运行 `make analytics-spec` 生成 `/analytics-spec.json`。
- 组件：`web/src/components/analytics/GameTypeInfo.tsx`
- 用法示例：
  ```tsx
  import GameTypeInfo from '@/components/analytics/GameTypeInfo';

  export default function GameDetail() {
    // 假设从接口拿到 game.game_type = 'tower_defense'
    const gameTypeId = 'tower_defense';
    return <GameTypeInfo gameTypeId={gameTypeId} />;
  }
  ```
- 组件内容：名称/ID、summary、description、特征、别名、代表作、推荐指标（带中文名）与推荐事件、默认维度。

## 后台游戏管理页：游戏类型区块
- 组件：`GameTypeSelectCard` + `GameTypeInfo` + 可选 `MetricsCatalogModal`
- 示例：
  ```tsx
  import GameTypeSelectCard from '@/components/analytics/GameTypeSelectCard';
  import MetricsCatalogModal from '@/components/analytics/MetricsCatalogModal';
  import { updateGame } from '@/services/games';

  export default function GameAdminSection({ game }: { game: { id: number; game_type?: string } }) {
    const [open, setOpen] = useState(false);
    return (
      <>
        <GameTypeSelectCard
          gameTypeId={game.game_type}
          onSave={async (next) => {
            // 后端若支持，直接保存到 /api/games/:id
            await updateGame(game.id, { game_type: next });
          }}
        />
        <a onClick={() => setOpen(true)}>查看指标目录</a>
        <MetricsCatalogModal open={open} onClose={() => setOpen(false)} />
      </>
    );
  }
  ```
- Demo：打开 `/dev/analytics-types` 预览所有类型卡片。

### 进阶：后端未支持 game_type 时的兜底存储
- 使用 `/api/configs` 存储游戏元信息（ID=`game.meta`，game_id=数值ID，env=空）。
- 服务封装：`web/src/services/gameMeta.ts` 提供 `loadGameMeta` / `saveGameMeta`。
- 组件 `GameTypeSelectCard` 内置了兜底 `saveGameMeta` 调用；也可在页面加载时用 `loadGameMeta` 预填 game_type。
- 等后端 `/api/games` 接口扩展出 `game_type/genre_code` 字段后，优先使用 `updateGame` 持久化，`configs` 作为备份。

### 在游戏管理列表显示类型/分类代码，并支持编辑
- 列展示：使用 `GameTypeTag` 渲染 `record.game_type`，直接加两列：
  - `游戏类型`（GameTypeTag）
  - `分类代码`（genre_code）
- 选择卡片：`GameTypeSelectCard` 已支持 `genre_code` 下拉（基于 `taxonomy.yaml`），保存时同时提交 `game_type/genre_code`。
- 演示：打开 `/dev/games-table` 查看列表与编辑交互。
