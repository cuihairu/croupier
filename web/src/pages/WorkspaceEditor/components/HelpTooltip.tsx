import { Tooltip, Modal, Button, Space, Typography } from 'antd';
import { QuestionCircleOutlined, BookOutlined } from '@ant-design/icons';

const { Text, Paragraph } = Typography;

/** 帮助文档内容 */
const HELP_DOCS: Record<string, { title: string; content: string; example?: string }> = {
  // Tab 基础配置
  'tab.key': {
    title: 'Tab 标识符',
    content:
      'Tab 的唯一标识符，用于路由和状态管理。只能包含小写字母、数字、下划线和连字符，不能以数字开头。建议使用有意义的命名，如 "player_list"、"player_detail" 等。',
  },
  'tab.title': {
    title: 'Tab 显示名称',
    content:
      'Tab 在界面上显示的名称，会展示在顶部导航栏。建议使用简洁的中文名称，如"玩家列表"、"玩家详情"等。',
  },
  'tab.icon': {
    title: 'Tab 图标',
    content:
      'Tab 图标用于增强视觉识别。可以填写 Ant Design Icons 的图标名称（如 UserOutlined、AppstoreOutlined 等）或自定义图标路径。',
  },
  'tab.comment': {
    title: '配置注释',
    content:
      '为 Tab 配置添加注释说明，帮助团队成员理解此 Tab 的用途、特殊逻辑或注意事项。注释内容不影响运行时渲染，仅用于编辑时查看。',
  },

  // 布局类型
  'layout.type': {
    title: '布局类型',
    content:
      '选择此 Tab 使用的布局模式，决定了数据的展示和交互方式：\n• list - 列表布局，适合数据浏览和批量操作\n• form - 表单布局，用于数据录入\n• detail - 详情布局，用于只读信息展示\n• form-detail - 查询-详情组合，先查询再展示\n• kanban - 看板布局，可视化流程管理\n• timeline - 时间线布局，展示时间序列事件',
  },

  // 函数绑定
  'layout.listFunction': {
    title: '列表数据函数',
    content:
      '指定获取列表数据的函数。下拉列表会自动筛选出返回数组的函数（通常 operation 为 list）。绑定后可点击"自动推导"按钮自动生成列配置。',
  },
  'layout.submitFunction': {
    title: '表单提交函数',
    content:
      '指定处理表单提交的函数。下拉列表会自动筛选出用于数据变更的函数（通常 operation 为 create、update）。绑定后可点击"自动推导"按钮自动生成字段配置。',
  },
  'layout.detailFunction': {
    title: '详情数据函数',
    content:
      '指定获取详情数据的函数。下拉列表会自动筛选出返回单个对象的函数（通常 operation 为 query 或 read）。',
  },
  'layout.queryFunction': {
    title: '查询函数',
    content:
      'form-detail 布局的查询函数，用于根据输入条件检索数据。绑定后可点击"自动推导"按钮自动生成查询字段配置。',
  },

  // 列配置
  'column.key': {
    title: '列字段名',
    content:
      '列绑定的数据字段名，必须与函数返回数据的属性名匹配。例如函数返回 `{ id: 1, name: "张三" }`，则字段名应为 "name"。',
  },
  'column.title': {
    title: '列标题',
    content: '列表头显示的列名称。',
  },
  'column.render': {
    title: '列渲染方式',
    content:
      '控制列数据的显示方式：\n• text - 纯文本显示\n• datetime - 日期时间格式化\n• date - 日期格式化\n• status - 状态标签（需要 renderOptions 配置映射）\n• tag - 标签显示\n• link - 链接样式\n• image - 图片展示\n• money - 金额格式化',
  },
  'column.width': {
    title: '列宽',
    content: '设置列的固定宽度（像素），留空则自动计算。',
  },
  'column.fixed': {
    title: '列固定',
    content: '在横向滚动时固定列的位置：\n• left - 左侧固定\n• right - 右侧固定\n• 留空 - 不固定',
  },
  'column.align': {
    title: '对齐方式',
    content:
      '列内容的对齐方式：\n• left - 左对齐（文本默认）\n• center - 居中对齐\n• right - 右对齐（数值默认）',
  },
  'column.ellipsis': {
    title: '超长省略',
    content: '启用后，超长文本会显示省略号，鼠标悬停时显示完整内容。',
  },
  'column.sortable': {
    title: '可排序',
    content: '启用后，点击列头可对该列进行排序。后端需要支持对应的排序参数。',
  },
  'column.visiblePermission': {
    title: '权限控制',
    content: '填写权限标识，用户无该权限时此列自动隐藏。如: workspace:edit',
  },
  'column.visibleCondition': {
    title: '数据条件',
    content:
      "填写 JS 表达式，运行时根据行数据判断是否显示此列。表达式返回 false 时隐藏。如: context.status !== 'archived'",
  },

  // 字段配置
  'field.key': {
    title: '字段名',
    content: '字段的唯一标识符，对应函数输入参数的属性名。',
  },
  'field.label': {
    title: '字段标签',
    content: '表单中字段显示的标签名称。',
  },
  'field.type': {
    title: '字段类型',
    content:
      '选择表单控件的类型：\n• input - 单行文本输入\n• number - 数字输入\n• select - 下拉选择\n• radio - 单选框\n• checkbox - 多选框\n• date - 日期选择\n• datetime - 日期时间选择\n• textarea - 多行文本\n• switch - 开关',
  },
  'field.required': {
    title: '必填',
    content: '开启后，用户必须填写此字段才能提交表单。',
  },
  'field.placeholder': {
    title: '占位符',
    content: '输入框为空时显示的提示文本，用于引导用户输入。',
  },
  'field.defaultValue': {
    title: '默认值',
    content:
      '字段的默认值，用户未填写时使用的预设值。可以设置静态值或使用表达式。点击"表达式"按钮可切换到表达式模式，支持动态值如当前时间、用户信息等。\n\n可用表达式：\n• $now() - 当前日期时间\n• $today() - 当前日期\n• $user.id - 当前用户ID\n• $user.name - 当前用户名\n• $query.key - URL查询参数\n• $localStorage.key - 本地存储\n• $uuid() - 生成UUID\n• $timestamp() - 当前时间戳',
  },
  'field.tooltip': {
    title: '帮助提示',
    content: '在字段标签旁显示的提示图标，鼠标悬停时展示的帮助文本，用于解释字段用途。',
  },
  'field.options': {
    title: '选项列表',
    content: 'select、radio、checkbox 类型字段的选项配置。每个选项包含显示标签和实际值。',
  },
  'field.rules': {
    title: '校验规则',
    content:
      '字段值的验证规则：\n• string - 字符串类型校验\n• number - 数字类型校验\n• email - 邮箱格式校验\n• url - URL 格式校验\n• pattern - 正则表达式校验\n可配置 min/max 值范围和错误提示信息。',
  },
  'field.visibleWhen': {
    title: '显隐条件',
    content:
      "填写 JS 表达式，运行时根据其他字段值判断是否显示此字段。表达式返回 false 时隐藏。如: status === 'active'",
  },
  'field.disabledWhen': {
    title: '禁用条件',
    content:
      "填写 JS 表达式，运行时根据其他字段值判断是否禁用此字段。表达式返回 true 时禁用。如: type === 'readonly'",
  },

  // 分组配置
  'ui:groups': {
    title: '字段分组',
    content:
      '将相关字段组织在一起，提升表单的可读性和用户体验。分组后，字段会按分组展示，每个分组可折叠展开。',
  },
};

interface HelpTooltipProps {
  helpKey: string;
  placement?: 'top' | 'right' | 'bottom' | 'left';
}

/** 帮助提示 Tooltip */
export function HelpTooltip({ helpKey, placement = 'right' }: HelpTooltipProps) {
  const doc = HELP_DOCS[helpKey];
  if (!doc) return null;

  return (
    <Tooltip
      placement={placement}
      title={
        <div style={{ maxWidth: 320 }}>
          <div style={{ fontWeight: 500 }}>{doc.title}</div>
          <div style={{ marginTop: 4, whiteSpace: 'pre-wrap', fontSize: 12 }}>{doc.content}</div>
          {doc.example && (
            <div style={{ marginTop: 4 }}>
              <Text type="secondary" style={{ fontSize: 11 }}>
                示例:{' '}
              </Text>
              <Text code style={{ fontSize: 11 }}>
                {doc.example}
              </Text>
            </div>
          )}
        </div>
      }
    >
      <QuestionCircleOutlined style={{ color: '#999', cursor: 'help' }} />
    </Tooltip>
  );
}

/** 帮助文档 Modal */
interface HelpModalProps {
  visible: boolean;
  onClose: () => void;
}

export function HelpModal({ visible, onClose }: HelpModalProps) {
  return (
    <Modal
      title={
        <Space>
          <BookOutlined />
          配置说明
        </Space>
      }
      open={visible}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width={640}
    >
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <div>
          <Text strong>Tab 基础配置</Text>
          <Paragraph style={{ fontSize: 12 }}>
            配置 Tab 的基本属性，包括标识符、显示名称、图标等。
          </Paragraph>
        </div>
        <div>
          <Text strong>布局类型</Text>
          <Paragraph style={{ fontSize: 12 }}>
            选择合适的布局类型来展示和交互数据。不同布局适用于不同场景，如列表适合数据浏览，表单适合数据录入。
          </Paragraph>
        </div>
        <div>
          <Text strong>函数绑定</Text>
          <Paragraph style={{ fontSize: 12 }}>
            将后端函数绑定到 Tab，系统会根据函数的 operation 类型自动推荐合适的布局类型。
          </Paragraph>
        </div>
        <div>
          <Text strong>列/字段配置</Text>
          <Paragraph style={{ fontSize: 12 }}>
            配置列表列和表单字段的显示方式、验证规则、联动关系等。
          </Paragraph>
        </div>
        <div style={{ background: '#f6f6f6', padding: 12, borderRadius: 4 }}>
          <Text strong>快捷操作</Text>
          <ul style={{ fontSize: 12, marginTop: 8 }}>
            <li>点击"自动推导"按钮根据函数 schema 生成布局</li>
            <li>点击"一键补全"补全当前布局的缺失配置</li>
            <li>点击"编排向导"进行多函数编排</li>
          </ul>
        </div>
      </Space>
    </Modal>
  );
}
