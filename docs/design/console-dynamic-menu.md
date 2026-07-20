# 运行控制台动态分类路由

## 结论

运行控制台按分类动态生成菜单和访问路径。

分类确定规则只有一套：

1. 工作台配置显式声明 `category` 时，使用 `category` 作为分类。
2. 未声明 `category` 时，使用 `objectKey` 的第一个 `.` 前缀作为分类。
3. `objectKey` 没有 `.` 时，整个 `objectKey` 就是分类。

示例：

| `category` | `objectKey`       | 最终分类 |
| ---------- | ----------------- | -------- |
| `support`  | `player.ban`      | support  |
| -          | `player.ban`      | player   |
| -          | `mail.send`       | mail     |
| -          | `mail`            | mail     |
| `ops`      | `server.restart`  | ops      |

## 确定时机

分类可以在工作台配置保存时由用户显式填写 `category`。

运行态加载已发布工作台后，再按同一套规则解析最终分类。菜单、分类页和工作台跳转都必须使用同一个解析函数，不能在页面里各自推断。

## 路由

运行控制台保留固定的参数路由承载动态菜单：

```text
/console/home
/console/:categoryKey
/console/:categoryKey/:objectKey
```

菜单结构：

```text
运行控制台
├── player
│   └── player.ban
├── mail
│   └── mail.send
└── support
    └── player.ban
```

`/console/:categoryKey` 展示该分类下的已发布工作台。

`/console/:categoryKey/:objectKey` 渲染具体工作台。如果地址中的分类和工作台配置解析出的分类不一致，前端应跳转到规范路径。

## 边界

不维护硬编码分类表。

不使用未知分类兜底。

不从前端页面里重复实现分类推断。

不把工作台列表塞进全局初始状态阻塞应用启动。

运行控制台只展示已发布工作台。页面装配、保存、发布仍在对象工作台完成。
