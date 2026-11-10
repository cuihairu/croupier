---
title: 生成式表单组件复用——契约基线与 RJSF 迁移评估
---

# 生成式表单组件复用——契约基线与 RJSF 迁移评估

日期：2026-10-27。状态：**评估记录 / D1 组件复用首片**（父任务是接口生成链路收口；`todo.md` 父行保持未勾，本片作为其证据记录）。

## 1. 范围与授权

接口生成链路要复用 Ant Design/ProComponents 的表格、表单、详情、对话框与状态组件，把定制工作收敛到业务语义、函数绑定与执行治理。本片覆盖该授权中的**表单复用**部分（D1）：

1. 为 ProComponents 表单容器 `ModalForm`、`DrawerForm`、`StepsForm` 建立**契约测试基线**。它们是生成式表单的复用目标；基线锁住其渲染、数据与校验契约，使后续迁移或 ProComponents 升级无法悄悄破坏既有生成表单。
2. 对把当前 JSON-Schema（RJSF）生成表单迁往这些容器做**首片评估**。RJSF 面积很大；按授权要求（"面积过大则记录评估、不硬迁移"），本片只记录评估结论，不迁移既有表单。

本片不改动任何既有页面/渲染器行为。

## 2. 当前生成表单实现

生成的业务页面通过 `web/src/components/SchemaFormRenderer`（一个 `@rjsf/antd` 封装）渲染表单。契约是 `FormPresentationSpec`（`jsonSchema` + `layout` + 呈现字段）；RJSF `uiSchema` 在内存中派生、从不持久化。

RJSF 生成表单的现有消费方：

- `web/src/components/PageRenderer/ResourcePageRenderer.tsx` —— Ant Design `Modal` 内的新建/编辑/行动作表单（`SchemaFormRenderer` 配 `hideSubmit`；Modal 的 `onOk` 校验并经 renderer ref 读值）。
- `web/src/components/PageRenderer/{CompositeRenderer,OperationPageRenderer,ReportPageRenderer,TaskPageRenderer}.tsx` —— 分节/详情/操作表单。
- `web/src/pages/PageStudio/CompositeEditor/PreviewNode.tsx` —— `ModalForm` 封装（本地组件，复用 `SchemaFormRenderer`，不是 ProComponents 的 `ModalForm`）与 `StaticFormLive`。
- `web/src/pages/Assignments/CloneModal.tsx`、`web/src/pages/Functions/Invoke/RequestBodyEditor.tsx`、`web/src/pages/PageStudio/CompositeEditor/PropsPanel.tsx` —— 各处零散 schema 表单。

具体模板样例：`web/src/components/PageRenderer/templates/*.tsx` 的 `createForm`/`updateForm` 与逐动作 `form`（如 `banManagement`、`mailCompensation`、`playerManage`）。

## 3. 复用目标与契约基线

`ModalForm`、`DrawerForm`、`StepsForm` 是 `@ant-design/pro-components` 的表单容器，把容器（弹窗/抽屉/分步）与 `ProForm` 提交器和 `onFinish` 取值契约合为一体。

新增契约测试：`web/src/components/FormContract/__tests__/formContainers.contract.test.tsx` 加快照，锁定：

- **渲染契约** —— 容器标题、声明字段、提交/取消按钮齐备（每容器一份快照）。
- **数据契约** —— `onFinish` 收到表单值；`ModalForm` 成功提交后关闭（`onOpenChange(false)`）；`StepsForm` 跨步保留早先步骤的值，最后一步提交合并值。
- **校验契约** —— 必填字段留空时阻止提交且不调用 `onFinish`。

这是 web 包的首批 jest 快照；快照序列化器剥离生成的 `css-var-_r_*` 哈希，同一依赖集内样式 token 哈希重建不会造成回归门禁假红，同时仍能捕获真实结构变化。

门禁：该文件在 `web` jest 下通过，且 `web` 全量 lint 保持绿。

## 4. RJSF Schema 迁移评估

### 4.1 候选映射

RJSF 的 `SchemaFormRenderer` 是自包含 `<Form>`（自带状态、自带提交器、AJV 校验、`uiSchema` 驱动控件）。ProComponents 容器期望 `ProForm` 子项并把值汇入单一 `onFinish`。简单的生成表单（扁平对象 schema、`string`/`number`/`enum` 字段、`required`）映射干净：

- `Modal` + `SchemaFormRenderer(hideSubmit)` → `ModalForm` 配 `ProFormText`/`ProFormSelect` 子项。
- `Drawer` 详情/编辑 → `DrawerForm`。
- 多步/任务表单 → `StepsForm`，每步一个 `StepForm`。

ProComponents 还提供 `BetaSchemaForm`（JSON schema → ProForm 字段），是最接近的自动桥。它是显式 **beta**（"API 可能变更"）。

### 4.2 阻抗与缺口

迁移面大于 happy path：当前渲染器具备的能力，纯 `ProForm`/`BetaSchemaForm` 今天并不覆盖：

- **自包含提交/ref 契约** —— `SchemaFormRendererHandle`（`submit`/`validate`/`getValues`）被 `Modal.onOk`/动作表单使用。迁往 `ModalForm` 会把提交所有权移交给容器，改变 `ResourcePageRenderer` 的调用点。
- **自定义控件** —— `web/src/components/SchemaFormRenderer/widgets.tsx` 与 `widgets-upload.tsx`（上传、远端选项下拉、呈现适配器）；没有控件注册表就无法表达为 `ProFormField` props。
- **条件可见性**（`visible-when`、`sectionCondition.ts`）与派生进 `uiSchema` 的**隐藏/排序字段**。
- **外部字段错误**（`extraErrors`，来自服务端校验）与自定义**错误边界**加重试——必须在容器包装层保留。
- **本地化 AJV 错误文案**与双语标签（`locales/*/schemaFormRenderer.ts`）。
- **只读/禁用**状态与模板驱动的表单结构。

`BetaSchemaForm` 今天不覆盖以上能力，且 beta 状态意味着迁移会钉死一个变动中的 API。把 RJSF 表单嵌进 `ModalForm`/`DrawerForm` 会产生两个表单实例、两条提交路径，不是复用收益。

### 4.3 结论

RJSF schema 表单是行为丰富、自包含的运行时；ProComponents 容器是外壳+提交契约。现在转换既有生成表单，要么把渲染器特性复制进 ProForm 字段 props，要么钉死 beta 桥——两者都超出本片，且可能把已绿的生成页面行为搞回归（5 个模板渲染器 + PageStudio 预览 + composite renderer + invoke 编辑器）。

**决定**：RJSF 生成表单继续作为运行时；把上述契约基线立为迁移护栏；硬迁移推迟到后续切片——要么 (a) 为已覆盖子集构建薄 `FormPresentationSpec → ProForm 字段` 适配层，要么 (b) 等 `BetaSchemaForm` 成熟后升级桥。本片不重写 12+ 个 `SchemaFormRenderer` 调用点。

## 5. 证据

- 契约测试 + 快照：`web/src/components/FormContract/__tests__/formContainers.contract.test.tsx`（ModalForm/DrawerForm/StepsForm 渲染 + 数据 + 校验契约）。
- 本评估记录：`docs/design/form-component-reuse.md`。
- 提交：见 `git log` 中该切片。
- 验证：新套件的 `web` jest，以及 web 全量 jest + `web` lint 门禁。
