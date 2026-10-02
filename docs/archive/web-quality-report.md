# Web 前端组件重复度分析报告（web/src）

> **已归档（2026-10-02 文档重整）**：本文是 2026-09-27 基线 `a6241f8` 的时点性只读
> 扫描报告，数据随代码演进而失效；后续重复度治理以此为准入参考，勿当现状引用。

- 扫描对象：`web/src` 下 `*.tsx`（排除 `__tests__/`、`*.test.tsx`、`*.d.ts`、`node_modules`、`.umi`）；附录覆盖 `web/src/services/api` 下 `*.ts`
- 扫描时基线：`a6241f8`（另有 3 个未提交文件 `docs/OPEN-ISSUES.md`、`web/src/app.tsx`、`web/tests/appAntdRoot.test.ts`，均不在任何相似对中）
- 扫描时间：2026-09-27；工具：python3（difflib + 自写正则 scanner，零安装、全程只读）
- 精确重复：按文件字节 sha256 分组
- 结构相似：规范化（剥注释；字符串字面量→`§`；非白名单标识符→`~`）后，相似度 = max(token 5-gram shingle Jaccard, 行级 SequenceMatcher ratio)；长度窗（0.6–1.67 倍）+ quick_ratio 预筛后全量 8846 对两两精算
- 阈值：主清单 ≥0.70 且双方 ≥40 行；另列 0.60–0.70 边界带与 0.50–0.60 观察带
- 重复行数：注释剥离后原始行 SequenceMatcher 公共行合计

## 1. 汇总

| 指标                                            | 值                      |
| ----------------------------------------------- | ----------------------- |
| 扫描文件数                                      | 247 tsx（+ 附录 45 ts） |
| 完全重复组（sha256）                            | **0**                   |
| 高相似对（≥0.70 且 ≥40 行）                     | **3**                   |
| 边界对（0.60–0.70）                             | 2                       |
| 观察对（0.50–0.60）                             | 9                       |
| 高相似对重复行合计                              | 499 行（402+39+58）     |
| 参与 ≥0.60 文件的重复行合计（文件级取最大匹配） | 1384 行                 |

**总体结论：无逐字节重复文件；克隆级重复集中在 1 个页面孪生对（Dev 热更/版本）与 1 组小组件对；PageStudio/CompositeEditor 存在一族声明式样板（0.50–0.75 带）；其余目录健康。**

## 2. 高相似对清单（≥0.70 且 ≥40 行，共 3 对）

`sh`=shingle Jaccard，`line`=盲化行级 SM ratio，`keep`=保留标识符变体 ratio（高值≈原文复制仅改字符串）。

| #   | 相似度    | sh / line / keep      | 文件 A（行数）                                    | 文件 B（行数）                                            | 重复行 | 结构差异                                                                                                                                                                                                  |
| --- | --------- | --------------------- | ------------------------------------------------- | --------------------------------------------------------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **0.853** | 0.853 / 0.832 / 0.716 | `pages/Dev/Hotpatches/index.tsx`（554）           | `pages/Dev/Releases/index.tsx`（576）                     | 402    | 复制粘贴孪生：ProTable+创建弹窗+状态流转+制品上传+灰度整套骨架逐段相同，仅热更域→版本域改名；状态机 action 集不同（approve/roll/applied/fail/rollback vs testing/gray/full/archive/rollback），属逻辑重复 |
| 2   | **0.816** | 0.650 / 0.816 / 0.735 | `pages/PageStudio/studio/PreviewDrawer.tsx`（51） | `components/ProposalInbox/ProposalPreviewModal.tsx`（52） | 39     | 同一「PageRenderer 只读预览 + 拒绝函数执行」包装的两个实例，仅 Drawer vs Modal、draft vs proposal.pageSpec、文案 key 不同                                                                                 |
| 3   | **0.747** | 0.459 / 0.747 / 0.733 | `CompositeEditor/components/Text.tsx`（80）       | `CompositeEditor/components/Button.tsx`（81）             | 58     | ComponentDef 声明骨架相同，字段集与枚举值不同（content/level vs title/btnStyle）                                                                                                                          |

**边界带（0.60–0.70）：**

| 相似度 | 文件对                                                                 | 重复行 | 说明                                                 |
| ------ | ---------------------------------------------------------------------- | ------ | ---------------------------------------------------- |
| 0.689  | CompositeEditor `Button.tsx` ↔ `Modal.tsx`                             | 63     | ComponentDef 族                                      |
| 0.636  | `components/PageEditor/OperationPageEditor.tsx` ↔ `TaskPageEditor.tsx` | 159    | Collapse + ResultView 列表编辑骨架共享，语义分段不同 |

**观察带（0.50–0.60，摘要）：** CompositeEditor `FnTable↔FnForm`(0.589)、`Text↔Modal`(0.589)、`FnFields↔FnTable/FnForm`(0.582/0.575)、`Container↔Modal`(0.528)、`Button↔FnForm`(0.503)；Support `FAQ↔Feedback`(0.589，223 行)、`FAQ↔Tickets`(0.515，248 行)、`Feedback↔Tickets`(0.489，282 行)；services/api `alerts↔nodes`(0.587)。

## 3. 按目录度量

参与重复 = 出现在 ≥0.60 相似对中的文件；重复行 = 每文件取最大公共行数。

| 目录                       | tsx 文件数 | 参与 ≥0.70 | 参与 0.60–0.70 | 观察 0.50–0.60 | 重复行合计      |
| -------------------------- | ---------- | ---------- | -------------- | -------------- | --------------- |
| `pages/Dev`                | 6          | 2          | 0              | 0              | 804             |
| `pages/PageStudio`         | 46         | 3          | 1              | 4              | 223             |
| `components/PageEditor`    | 7          | 0          | 2              | 0              | 318             |
| `components/ProposalInbox` | 6          | 1          | 0              | 0              | 39              |
| `pages/Support`            | 5          | 0          | 0              | 3              | —（观察带不计） |
| `pages/Functions`          | 24         | 0          | 0              | 0              | 0               |
| `pages/Ops`                | 18         | 0          | 0              | 0              | 0               |
| `pages/Analytics`          | 16         | 0          | 0              | 0              | 0               |
| 其余 30 个目录             | 122        | 0          | 0              | 0              | 0               |

刚并入 main 的 `Functions/Directory`、`Functions/SdkDistribution`、`OpenAPISources` 无 ≥0.50 相似对。

## 4. 分组合并方案

### 簇 A（P1，无在途）：Dev 热更/版本「单资源+状态机」页面族

- 成员：`pages/Dev/Hotpatches/index.tsx`、`pages/Dev/Releases/index.tsx` + 同族 `services/api/hotpatches.ts` ↔ `releases.ts`（0.747）
- 形态：配置驱动 `StatefulResourcePage`（`{ columns, statusLabels/Colors, transitions, createFields, artifactUpload }`），或最小动作版先抽 `useResourceTransition` / `useArtifactUpload` 两个 hook + 共享 columns 构建器；service 层同批抽象
- 预期削减 ~400 行；状态机 action 集不同，须配置化而非参数分支
- P1 依据：0.853 ≥0.85 且 pages/Dev 无在途会话

### 簇 B（P2，在途避让）：PageSpec 只读预览容器

- 成员：`PageStudio/studio/PreviewDrawer.tsx` ↔ `ProposalInbox/ProposalPreviewModal.tsx`
- 形态：`components/PageSpecPreview`（`{ open, pageSpec, onClose, variant: 'drawer'|'modal', width? }`）
- **PreviewDrawer 位于 wt-pages 活动区，暂缓**

### 簇 C（P2，在途避让）：CompositeEditor ComponentDef 声明族

- 成员：`CompositeEditor/components/` 下 7/10 文件互为 0.50–0.75
- 形态：不合并文件，抽 `defineStringProp` / `defineEnumProp` 等 propSchema 构建帮手 + 共享 ComponentDef 骨架工厂
- **全部位于 wt-pages 活动区，暂缓**

### 簇 D（P2，无在途但低于 0.70 门槛）：PageEditor 语义编辑器骨架

- 成员：`OperationPageEditor.tsx` ↔ `TaskPageEditor.tsx`（0.636，159 行）
- 形态：抽 `ResultViewListEditor`（`{ value, onChange }`，内封装 SortableList + LocalizedTextEditor）

### 观察项（不计入合并计划）

- `pages/Support/FAQ|Feedback|Tickets`（0.489–0.589，223–282 行）：骨架相似但逻辑已分叉，且属 wt-support 待合并族——**落地后复扫再决定是否抽 SupportCrudPage**

### 在途避让清单

| 区域                     | 在途                    | 处置                      |
| ------------------------ | ----------------------- | ------------------------- |
| `pages/Assignments/**`   | wt-support 正在写       | 暂不合并（当前无对象）    |
| `pages/Support/**`       | wt-support 已提交待合并 | **暂不合并**，合并后复扫  |
| `pages/Ops/Schedules/**` | wt-support              | 暂不动（最高 0.407）      |
| `pages/PageStudio/**`    | wt-pages 有未跟踪新测试 | **谨慎/暂缓**，落地后执行 |

## 5. 已知边界

1. 非 AST 分析（正则级 scanner）：正则字面量、模板串内嵌 `${}` 按整体字符串占位，极端情况可能误切。
2. 语义重复漏判：HOC、render props、动态 import、schema 分发造成的「语义重复、文本相异」不在检出范围。
3. 样式差异被占位化吞掉：高相似不代表视觉一致；纯注释复制剥注释后不单独计入。
4. 重排敏感：行级 SM 对段落换序低估；<0.50 的「骨架级共享」（如各 ProTable 页共用查询-表格骨架）未列入——那应走组件库抽象而非去重叙事。
5. 范围：仅静态扫描 `web/src`；locales 文案重复、测试代码重复、`web/src` 之外目录未覆盖。
