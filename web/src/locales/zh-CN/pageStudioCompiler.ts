// pages.pageStudio.compiler.* — CompositeEditor 编译器（compile/decompile）诊断警告
// 与 types（视图元数据/参数化候选属性标签）
export default {
  'pages.pageStudio.compiler.paramProp.autoRun': '自动执行',
  'pages.pageStudio.compiler.paramProp.span': '栅格宽度',
  'pages.pageStudio.compiler.paramProp.title': '标题',
  'pages.pageStudio.compiler.view.actions.hint': '无参动作，点击即执行',
  'pages.pageStudio.compiler.view.actions.label': '按钮组',
  'pages.pageStudio.compiler.view.fields.hint': '键值详情，单对象展示',
  'pages.pageStudio.compiler.view.fields.label': '字段卡',
  'pages.pageStudio.compiler.view.form.hint': '输入参数执行操作',
  'pages.pageStudio.compiler.view.form.label': '操作表单',
  'pages.pageStudio.compiler.view.table.hint': '列表查询，展示多行数据',
  'pages.pageStudio.compiler.view.table.label': '表格',
  'pages.pageStudio.compiler.warning.buttonAfterTable':
    '按钮「{title}」需放置在表格之后（编译为表格顶部按钮），已忽略',
  'pages.pageStudio.compiler.warning.buttonEmptyModalTarget':
    '按钮「{title}」的弹窗目标无效（空弹窗），已忽略',
  'pages.pageStudio.compiler.warning.buttonModalTargetInvalid':
    '按钮「{title}」的弹窗目标无效，已忽略',
  'pages.pageStudio.compiler.warning.buttonNoAction': '按钮「{title}」没有配置动作，已忽略',
  'pages.pageStudio.compiler.warning.buttonTargetInvalid': '按钮「{title}」动作目标无效，已忽略',
  'pages.pageStudio.compiler.warning.buttonTargetLost':
    '按钮「{label}」的弹窗目标 {target} 无法还原，已丢弃',
  'pages.pageStudio.compiler.warning.emptyModal': '弹窗「{title}」为空，已忽略',
  'pages.pageStudio.compiler.warning.emptyTabs': '页签容器「{title}」为空，已忽略',
  'pages.pageStudio.compiler.warning.expressionUnknownVariable':
    '区块「{title}」参数「{param}」的表达式「{value}」引用未知变量或行上下文，已按字面量保存',
  'pages.pageStudio.compiler.warning.invalidStaticSchema':
    '常量表单「{title}」的 JSON 定义无效，已跳过',
  'pages.pageStudio.compiler.warning.mappingSourceInvalid':
    '区块「{title}」参数「{param}」的来源节点已失效，已跳过',
  'pages.pageStudio.compiler.warning.mappingSourceMissing':
    '区块「{ownerKey}」参数映射来源「{sourceKey}」不存在，已保留字面值',
  'pages.pageStudio.compiler.warning.missingFunctionBinding': '组件「{title}」没有绑定函数，已忽略',
  'pages.pageStudio.compiler.warning.rowActionMissingTarget':
    '表格「{title}」有未配置目标的行操作，已忽略',
  'pages.pageStudio.compiler.warning.rowActionParamNestedPath':
    '表格「{title}」行操作参数「{param}」的嵌套字段「{path}」发布后无法求值，已按字面量保存',
  'pages.pageStudio.compiler.warning.rowActionParamRowOnly':
    "表格「{title}」行操作参数「{param}」的表达式「{value}」仅支持 '{{'row.字段'}}'，已按字面量保存",
  'pages.pageStudio.compiler.warning.rowActionTargetLost':
    '行操作「{label}」的弹窗目标 {target} 无法还原，已丢弃',
  'pages.pageStudio.compiler.warning.sectionKeyInvalid':
    '区块「{title}」的 key「{key}」非法或重复，已自动分配',
  'pages.pageStudio.compiler.warning.sectionMissingFunction': '区块缺少函数绑定，已跳过',
  'pages.pageStudio.compiler.warning.successRefreshLost':
    '「成功后刷新」引用 {target} 无法还原，已丢弃',
  'pages.pageStudio.compiler.warning.textSkipped': '文本「{content}」不参与发布（V1）',
  'pages.pageStudio.compiler.warning.unknownMappingKind':
    '区块「{title}」参数「{param}」的映射类型「{kind}」未知，已跳过',
  'pages.pageStudio.compiler.warning.unknownNodeType': '未知组件类型 {type}，已忽略',
};
