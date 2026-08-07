/**
 * P0-0 验收测试：真实游戏 JSON Schema 表单验证
 *
 * 验证 SchemaFormRenderer 能正确处理游戏运营场景的复杂 JSON Schema
 */

import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';

// 真实游戏 JSON Schema 示例
const PLAYER_BAN_SCHEMA = {
  type: 'object',
  properties: {
    playerId: { type: 'string', title: '玩家ID' },
    reason: { type: 'string', title: '封禁原因', minLength: 1 },
    duration: {
      type: 'string',
      title: '封禁时长',
      enum: ['1h', '24h', '7d', '30d', 'permanent'],
      enumNames: ['1小时', '24小时', '7天', '30天', '永久'],
    },
    evidence: {
      type: 'array',
      title: '证据截图',
      items: { type: 'string', format: 'uri' },
    },
  },
  required: ['playerId', 'reason', 'duration'],
};

const MAIL_SEND_SCHEMA = {
  type: 'object',
  properties: {
    recipients: {
      type: 'array',
      title: '收件人',
      items: { type: 'string' },
      minItems: 1,
    },
    subject: { type: 'string', title: '主题', maxLength: 100 },
    content: { type: 'string', title: '内容', format: 'textarea' },
    attachments: {
      type: 'array',
      title: '附件',
      items: {
        type: 'object',
        properties: {
          name: { type: 'string' },
          url: { type: 'string', format: 'uri' },
        },
      },
    },
    scheduledAt: { type: 'string', title: '定时发送', format: 'date-time' },
  },
  required: ['recipients', 'subject', 'content'],
};

const REWARD_GRANT_SCHEMA = {
  type: 'object',
  properties: {
    playerIds: {
      type: 'array',
      title: '玩家列表',
      items: { type: 'string' },
      minItems: 1,
      maxItems: 1000,
    },
    rewards: {
      type: 'array',
      title: '奖励配置',
      items: {
        type: 'object',
        properties: {
          itemId: { type: 'string', title: '物品ID' },
          quantity: { type: 'integer', title: '数量', minimum: 1 },
        },
        required: ['itemId', 'quantity'],
      },
    },
    reason: { type: 'string', title: '发放原因' },
    expireAt: { type: 'string', title: '过期时间', format: 'date-time' },
  },
  required: ['playerIds', 'rewards', 'reason'],
};

const ANALYTICS_QUERY_SCHEMA = {
  type: 'object',
  properties: {
    startDate: { type: 'string', title: '开始日期', format: 'date' },
    endDate: { type: 'string', title: '结束日期', format: 'date' },
    metrics: {
      type: 'array',
      title: '指标',
      items: {
        type: 'string',
        enum: ['dau', 'mau', 'revenue', 'retention', 'arpu'],
      },
    },
    dimensions: {
      type: 'array',
      title: '维度',
      items: {
        type: 'string',
        enum: ['channel', 'platform', 'region', 'level_range'],
      },
    },
    filters: {
      type: 'object',
      title: '筛选条件',
      properties: {
        channel: { type: 'string' },
        platform: { type: 'string' },
      },
    },
  },
  required: ['startDate', 'endDate', 'metrics'],
};

describe('P0-0: 真实游戏 JSON Schema 验证', () => {
  const user = userEvent.setup();

  test('玩家封禁表单 - 基础字段渲染', async () => {
    const onSubmit = jest.fn();
    render(<SchemaFormRenderer schema={PLAYER_BAN_SCHEMA} onSubmit={onSubmit} />);

    // 验证必填字段渲染
    expect(screen.getByLabelText(/玩家ID/)).toBeInTheDocument();
    expect(screen.getByLabelText(/封禁原因/)).toBeInTheDocument();
    expect(screen.getByLabelText(/封禁时长/)).toBeInTheDocument();
  });

  test('玩家封禁表单 - 枚举字段渲染', async () => {
    const onSubmit = jest.fn();
    render(<SchemaFormRenderer schema={PLAYER_BAN_SCHEMA} onSubmit={onSubmit} />);

    // 验证枚举字段
    const durationSelect = screen.getByLabelText(/封禁时长/);
    expect(durationSelect).toBeInTheDocument();
  });

  test('玩家封禁表单 - 必填验证', async () => {
    const onSubmit = jest.fn();
    render(<SchemaFormRenderer schema={PLAYER_BAN_SCHEMA} onSubmit={onSubmit} />);

    // 直接提交不填值
    const submitBtn = screen.getByRole('button', { name: /提交|submit/i });
    await user.click(submitBtn);

    // 验证错误提示
    await waitFor(() => {
      expect(screen.getByText(/请输入|required/i)).toBeInTheDocument();
    });
    expect(onSubmit).not.toHaveBeenCalled();
  });

  test('玩家封禁表单 - 正常提交', async () => {
    const onSubmit = jest.fn();
    render(<SchemaFormRenderer schema={PLAYER_BAN_SCHEMA} onSubmit={onSubmit} />);

    // 填写表单
    await user.type(screen.getByLabelText(/玩家ID/), '1001');
    await user.type(screen.getByLabelText(/封禁原因/), '使用外挂');

    // 提交
    const submitBtn = screen.getByRole('button', { name: /提交|submit/i });
    await user.click(submitBtn);

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalled();
    });
  });

  test('邮件发送表单 - 数组字段渲染', async () => {
    const onSubmit = jest.fn();
    render(<SchemaFormRenderer schema={MAIL_SEND_SCHEMA} onSubmit={onSubmit} />);

    // 验证数组字段
    expect(screen.getByLabelText(/收件人/)).toBeInTheDocument();
    expect(screen.getByLabelText(/主题/)).toBeInTheDocument();
    expect(screen.getByLabelText(/内容/)).toBeInTheDocument();
  });

  test('批量发奖表单 - 嵌套对象渲染', async () => {
    const onSubmit = jest.fn();
    render(<SchemaFormRenderer schema={REWARD_GRANT_SCHEMA} onSubmit={onSubmit} />);

    // 验证嵌套字段
    expect(screen.getByLabelText(/玩家列表/)).toBeInTheDocument();
    expect(screen.getByLabelText(/奖励配置/)).toBeInTheDocument();
    expect(screen.getByLabelText(/发放原因/)).toBeInTheDocument();
  });

  test('数据分析查询 - 日期格式渲染', async () => {
    const onSubmit = jest.fn();
    render(<SchemaFormRenderer schema={ANALYTICS_QUERY_SCHEMA} onSubmit={onSubmit} />);

    // 验证日期字段
    expect(screen.getByLabelText(/开始日期/)).toBeInTheDocument();
    expect(screen.getByLabelText(/结束日期/)).toBeInTheDocument();
    expect(screen.getByLabelText(/指标/)).toBeInTheDocument();
  });

  test('复杂嵌套 schema - 中文错误提示', async () => {
    const schema = {
      type: 'object',
      properties: {
        name: { type: 'string', title: '名称', minLength: 2, maxLength: 50 },
        age: { type: 'integer', title: '年龄', minimum: 0, maximum: 150 },
      },
      required: ['name'],
    };

    const onSubmit = jest.fn();
    render(<SchemaFormRenderer schema={schema} onSubmit={onSubmit} />);

    // 填写无效值
    await user.type(screen.getByLabelText(/名称/), 'a');

    // 提交
    const submitBtn = screen.getByRole('button', { name: /提交|submit/i });
    await user.click(submitBtn);

    // 验证中文错误提示
    await waitFor(() => {
      expect(screen.getByText(/至少|最少|minLength/i)).toBeInTheDocument();
    });
  });
});
