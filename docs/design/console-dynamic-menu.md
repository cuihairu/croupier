# 运行控制台动态菜单

> **状态**：Current — 运行控制台菜单只消费已发布 PageSpec。详细模型见 [Dashboard Resource/Page 模型](../architecture/dashboard-page-model.md)。

## 结论

运行控制台左侧菜单不是静态路由配置，也不是函数目录的直接投影。

菜单来源只有一个：

```text
PublishedPageSpec[] -> ConsoleMenuSpec
```

前端不得为动态分类修改 `web/src/locales/*/menu.ts`。动态分类和页面标题必须分别来自已发布 PageSpec 的 `category.labels` 与 `title`，而不是不受约束的 metadata。

## 分类规则

分类 key 的确定规则只有一套：

1. PageSpec 显式声明 `category.key` 时，使用该值。
2. 未声明分类时，使用 `resourceKey` 的第一个 `.` 前缀。
3. 没有 `resourceKey` 时，使用 `pageKey` 的第一个 `.` 前缀。
4. 没有 `.` 时，整个 `resourceKey` 或 `pageKey` 就是分类。

示例：

| 输入 | 最终分类 |
| --- | --- |
| `category.key = support`, `resourceKey = player` | `support` |
| `resourceKey = player.ban` | `player` |
| `resourceKey = mail.send` | `mail` |
| `resourceKey = mail` | `mail` |
| `pageKey = analytics.retention` | `analytics` |

## 确定时机

函数注册不提供运行菜单分类，也不提供分类多语言显示名。Server 可以根据 `resourceKey`、`pageKey` 和契约分析给 Page Studio 提供分类建议，但最终分类必须在 PageSpec 保存或发布时确定。

运行控制台加载菜单时不再推断分类。它只能读取已经发布并通过校验的 `category.key`、`category.labels` 和页面 labels。

## 多语言

动态菜单显示名从 PageSpec 的强类型字段中取值：

```json
{
  "category": {
    "key": "support",
    "labels": {
      "zh-CN": "客服",
      "en-US": "Support"
    }
  },
  "title": {
    "zh-CN": "封禁玩家",
    "en-US": "Ban Player"
  }
}
```

规则：

- `category.labels` 用于分类菜单标题。
- `title` 用于页面菜单标题。
- 静态 locale 只用于固定系统菜单，例如“运行控制台”。
- 动态菜单项必须设置 `locale: false`。
- 缺少系统默认语言时发布失败。

## 路由

运行控制台保留固定参数路由承载动态菜单：

```text
/console/home
/console/:categoryKey
/console/:categoryKey/:pageKey
```

`/console/:categoryKey` 展示该分类下的已发布页面。

`/console/:categoryKey/:pageKey` 渲染具体 PageSpec。如果地址中的分类和 PageSpec 发布分类不一致，前端应跳转到规范路径。URL 不是 scope：页面、菜单和执行都按全局 `game_id + env` context 查询；同一个 `pageKey` 可以存在于不同 scope。

## 边界

禁止：

- 维护硬编码分类表。
- 为动态分类新增静态 i18n key。
- 从前端页面里重复实现分类推断。
- 从函数目录直接生成运行控制台菜单。
- 把未发布 PageSpec 或函数注册草稿展示到运行控制台。
- 缺少分类 labels 时静默显示 key 并继续发布。

允许：

- PageSpec 保存时根据规则生成分类 key。
- Server 根据函数能力契约生成 PageSpec 建议；建议不是菜单事实源。
- 用户在 Page Studio 中覆盖分类、标题、图标和排序。

## 验收规则

- 新增分类不需要改前端代码。
- 切换语言后，动态分类和页面标题来自 PageSpec labels。
- 没有 PageSpec 发布时，运行控制台不展示对应菜单。
- 没有 `category.labels` 默认语言时发布失败。
- 函数目录、Page Studio 草稿和运行控制台菜单之间不存在第二套分类逻辑。
- 切换全局 game/env 后，菜单只显示新 scope 的 active PublishedPageSpec。
