---
title: 组合页编辑器 V3 使用与扩展指南
---

# 组合页编辑器 V3（组件化）

> 状态：已上线（P0–P5 全批次完成，验收记录见 [V3 计划](./composite-editor-v3-plan.md)）
> 参考样品：[amis-editor](http://192.168.5.5:8001)（scaffold/属性面板声明）、[Appsmith](http://192.168.5.5:8002)（一切皆组件/事件绑定）

## 用户指南

### 入口

提案收件箱「创建组合页」或 `/functions/pages/composite-editor`。**函数按 (game, env) 隔离**——当前 scope 无函数时左栏会引导切换到有函数的 scope。

### 三步搭「玩家管理页」

```
1. 左栏点击 player.list   → 画布出现表格（列=输出 schema，自动执行）
2. 左栏点「基础组件 → 弹窗」→ 弹窗收纳区出现空弹窗
   左栏点击 mail.send 拖到弹窗卡片上 → 表单装入弹窗
3. 选中表格 → 属性面板「行操作」→ 添加：
     文案=发邮件 | 打开弹窗=发邮件 | 参数映射 playerId ← 行.uid
   选中弹窗表单 → 「成功后刷新」= 刷新 player.list
```

点顶栏「预览」：表格自动执行出真实数据 → 行尾 `[发邮件]` → 弹窗（playerId 已带入）→ 提交 → 关窗 → 表格刷新。**预览即发布后形态**。

「保存为提案」→ 提案收件箱接受发布 → 左侧菜单出现页面。

### 画布操作

| 操作      | 方式                                                  |
| --------- | ----------------------------------------------------- |
| 添加      | 左栏点击（加到末尾）或拖拽（拖到指定落点/弹窗收纳区） |
| 排序      | 拖节点左上角手柄                                      |
| 调宽      | 拖节点右边缘（4–24 栅格）                             |
| 配置      | 点节点 → 右侧属性面板（rjsf schema 驱动）             |
| 复制/删除 | 节点右上角操作按钮                                    |
| 导航      | 左栏「大纲」Tab 组件树                                |

### 编译规则（保存时树 → CompositeSection）

| 画布                                     | 发布 spec                                        |
| ---------------------------------------- | ------------------------------------------------ |
| fnTable / fnFields / fnForm(inline)      | 区块（view=table/fields/form）                   |
| 弹窗容器 + fnForm                        | `display: dialog` 区块                           |
| 表格属性「行操作」                       | `table.rowActions`（行字段→表单参数映射）        |
| 独立按钮（onClick=打开弹窗，置于表格后） | 该表格 `toolbarActions`（V1 编译为表格顶部按钮） |
| fnForm「成功后刷新」                     | `onSuccessRefresh`                               |
| 文本                                     | 不发布（警告提示）                               |

## 组件开发指南

新组件 = 在 `components/builtin.tsx` 注册一个 `ComponentDef`：

```tsx
registerComponent({
  type: "statCard", // ComponentType 联合类型加一项
  name: "统计卡",
  icon: <Tag color="orange">统计</Tag>,
  category: "basic", // function=由契约生成 / basic=直接拖入
  allowedChildren: ["fnFields"], // 容器类才配（可选）
  propSchema: ({ nodes, fnById, fn }) => ({
    type: "object",
    properties: {
      title: { type: "string", title: "标题" },
      span: { type: "integer", minimum: 4, maximum: 24, default: 24 },
      // 事件字段：format:'action' → ActionEditor 下拉编排
      onClick: { type: "object", title: "点击动作", format: "action" },
    },
  }),
  scaffold: (fn) => ({ title: fn?.id ?? "统计", span: 12 }),
  Preview: ({ node, fn }) => <StatCardPreview node={node} fn={fn} />,
});
```

约定：

- **propSchema 即属性面板**：普通字段 rjsf 渲染；`format:'action'` → 动作编排；`format:'rowActions'` → 行操作编辑器；`actionKinds` 限定可选动作。
- **scaffold 按契约实例化**（amis 式）：函数组件从 FunctionContract 取输出列/输入字段作为默认值。
- **Preview 是画布预览**：不执行调用；数据展示留空态文案引导试跑/预览。
- 新组件若参与发布：`compiler.ts` 加编译规则 + 快照用例。

## 架构位置

```
编辑器（树模型 PageNode）          发布链（零改动复用）
┌ 组件面板/大纲（左）              ┌ GenerateCompositePage
│ registry: ComponentDef          │ spec.CompositeSection
│  ├ scaffold（amis）              │  ├ display/rowActions/toolbar
│  ├ propSchema（panelControls）   │  └ onSuccessRefresh
│  └ Preview                      │ PageRenderer.CompositeRenderer
├ 画布 Canvas（中）                │  ├ Modal 弹窗渲染
│  ├ DnD 拖入/重排/调宽            │  ├ 行操作列（参数预填）
│  └ modal 收纳区                  │  └ 成功刷新级联
├ 属性面板 PropsPanel（右）        └ 提案 → 发布 → 菜单
│  └ rjsf + ActionEditor
└ 预览 PreviewRuntime = 发布形态
   └ 保存 compiler: 树 → sections
```

V1 边界（V1.1 计划内）：容器单层、弹窗内单表单、无撤销重做/动作链/多选、独立按钮编译依赖表格、回读编辑（已发布页反编译为树）。
