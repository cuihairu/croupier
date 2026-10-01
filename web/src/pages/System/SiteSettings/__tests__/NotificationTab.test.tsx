/**
 * 通知设置子 Tab 单测（覆盖率巡检：0% → 行覆盖 100% → 分支翼收口）。
 *
 * 锁定契约：通知配置读（回填 + 密文永不回显）/写（trim、空值=清除、
 * 非字符串 undefined 直提不清 trim）/开关（开/关双翼文案）/三条失败路径
 * （加载/保存/操作）与渲染分支（密文徽标两态、placeholderMsg/help）。
 * SMTP/邮件开关已迁运维 SMTP 卡（#55），本文件不再覆盖；对应行为见
 * __tests__/SmtpCard.test.tsx。
 *
 * mock 口径沿用 __tests__/index.test.tsx：services/api/sites 三方法 jest.mock、
 * @umijs/max 本地 mock。message 提示经真实 antd App 渲染进 portal，用 DOM
 * 文本断言；开关无 accessible name，按 DOM 序索引定位（站内信在前）。
 *
 * 边界（诚实）：
 * 1. secretState 的 ternary 链缺 feishuSecret 特例：飞书密钥徽标显示
 *    webhookSecretSet/Masked（现状行为，用例按现状断言并标注翻转条件）。
 * 2. NotificationSettings.feishuSecretSet 字段存在但徽标不消费它（同 1）。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import NotificationTab from '../NotificationTab';
import type { NotificationSettings } from '@/services/api/sites';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sites', () => ({
  fetchNotificationSettings: jest.fn(),
  setSiteSetting: jest.fn(),
  clearSiteSetting: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
    values,
  }: {
    defaultMessage?: string;
    values?: Record<string, string>;
  }) => {
    let text = defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, String(v));
    }
    return <>{text}</>;
  },
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, String(v));
      }
      return text;
    },
  }),
}));

import { clearSiteSetting, fetchNotificationSettings, setSiteSetting } from '@/services/api/sites';

const mFetch = fetchNotificationSettings as jest.MockedFunction<typeof fetchNotificationSettings>;
const mSet = setSiteSetting as jest.MockedFunction<typeof setSiteSetting>;
const mClear = clearSiteSetting as jest.MockedFunction<typeof clearSiteSetting>;

const baseSettings: NotificationSettings = {
  emailEnabled: false,
  smtpHost: '',
  smtpPort: 0,
  smtpUser: '',
  smtpFrom: '',
  smtpPasswordSet: false,
  smtpEncryption: '',
  smtpAuthType: '',
  smtpInsecureSkipVerify: false,
  dingtalkUrl: '',
  dingtalkSecretSet: false,
  webhookUrl: '',
  webhookSecretSet: false,
  wecomUrl: '',
  feishuUrl: '',
  feishuSecretSet: false,
  inAppEnabled: true,
};

function renderTab() {
  return render(
    <App>
      <ConfigProvider>
        <NotificationTab />
      </ConfigProvider>
    </App>,
  );
}

/** 定位控件所在 Form.Item 内的「保存」按钮（页面存在多个同名按钮，禁止全局 [0]） */
function saveButtonOf(control: HTMLElement): HTMLElement {
  const item = control.closest('.ant-form-item');
  expect(item).not.toBeNull();
  return within(item as HTMLElement).getByRole('button', { name: '保存' });
}

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue({ ...baseSettings });
  mSet.mockResolvedValue(undefined);
  mClear.mockResolvedValue(undefined);
});

describe('NotificationTab 加载', () => {
  it('成功：回填非密文字段、密文不回显、密文徽标两态、站内信开关与迁移提示', async () => {
    mFetch.mockResolvedValue({
      ...baseSettings,
      emailEnabled: true,
      inAppEnabled: false,
      smtpHost: 'smtp.example.com',
      smtpUser: 'noreply@example.com',
      smtpPasswordSet: true,
      smtpPasswordMasked: '•••ab12',
      dingtalkUrl: 'https://oapi.dingtalk.com/robot?access_token=x',
      dingtalkSecretSet: false,
      webhookUrl: 'https://receiver.example.com/hook',
      webhookSecretSet: true,
      // webhookSecretMasked 缺省：走 masked ?? '' 右侧（「已配置 」无尾号）
      feishuSecretSet: false,
    });
    renderTab();

    // 非 URL 字段正常回显（兼作加载完成等待门）
    expect(
      await screen.findByDisplayValue('https://oapi.dingtalk.com/robot?access_token=x'),
    ).toBeInTheDocument();
    // SMTP 已迁运维（#55）：本 Tab 不再渲染 SMTP 字段
    expect(screen.queryByDisplayValue('smtp.example.com')).not.toBeInTheDocument();
    expect(screen.queryByText('SMTP 服务器')).not.toBeInTheDocument();
    // 密文字段永不回显（表单显式 setFieldsValue undefined；
    // 本 Tab 已无 '••••••••' 占位的密码框，密码框行为见 SmtpCard 测试）
    expect(screen.queryByPlaceholderText('••••••••')).not.toBeInTheDocument();

    // 迁移提示 + 开关只剩站内信=false
    expect(await screen.findByText(/已迁移至「运维」Tab/)).toBeInTheDocument();
    const [inApp] = screen.getAllByRole('switch');
    expect(inApp).not.toBeChecked();

    // 密文徽标（现状行为）：SMTP 密码徽标已随 #55 迁 SmtpCard；feishuSecret
    // 的徽标在组件 ternary 链里无专属分支、落 webhookSecretSet/Masked——
    // fixture 的 feishuSecretSet:false 不参与显示，「未配置」仅钉钉 1 个、
    // 无尾号「已配置 」为 webhook+飞书 2 个。组件若修正为读 feishuSecretSet，
    // 此处断言需同步翻转。
    expect(screen.queryByText('已配置 •••ab12')).not.toBeInTheDocument();
    expect(screen.getAllByText('未配置')).toHaveLength(1);
    expect(screen.getAllByText('已配置')).toHaveLength(2);
  });

  it('加载失败：错误提示走 extractErrorMessage 兜底文案，页面以 null settings 渲染', async () => {
    mFetch.mockRejectedValue(undefined); // 非 Error 对象：锁定 fallback 分支
    renderTab();

    expect(await screen.findByText('加载通知配置失败')).toBeInTheDocument();
    // settings=null：无密文徽标（f.secret && settings 右侧）
    expect(screen.queryByText('未配置')).not.toBeInTheDocument();
    // 开关落默认值：inAppEnabled ?? true（SMTP 已迁运维，无邮件开关）
    const [inApp] = screen.getAllByRole('switch');
    expect(inApp).toBeChecked();
  });
});

describe('NotificationTab 保存（saveKey）', () => {
  it('文本字段 trim 后提交 setSiteSetting，成功提示并重拉', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, dingtalkUrl: 'https://old' });
    renderTab();
    const input = await screen.findByDisplayValue('https://old');
    fireEvent.change(input, { target: { value: '  https://new  ' } });
    fireEvent.click(saveButtonOf(input));

    await waitFor(() =>
      expect(mSet).toHaveBeenCalledWith('notification.dingtalkUrl', 'https://new'),
    );
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    // 成功后重拉（初始 1 次 + 保存后 1 次）
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(mClear).not.toHaveBeenCalled();
  });

  it('空值提交 = 清除覆盖：走 clearSiteSetting 且不带 trim 产物', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, dingtalkUrl: 'https://old' });
    renderTab();
    const input = await screen.findByDisplayValue('https://old');
    fireEvent.change(input, { target: { value: '   ' } });
    fireEvent.click(saveButtonOf(input));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('notification.dingtalkUrl'));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mSet).not.toHaveBeenCalled();
  });

  it('非字符串空值（undefined，未触碰的密钥字段）：不经 trim 直落 clearSiteSetting', async () => {
    renderTab();
    // load 显式 setFieldsValue(secret: undefined)：字段未触碰时 getFieldValue
    // 恒 undefined → typeof 翼走非字符串侧（L176），trim 不适用
    const input = await screen.findByPlaceholderText('SEC…');
    fireEvent.click(saveButtonOf(input));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('notification.dingtalkSecret'));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mSet).not.toHaveBeenCalled();
    // 空值清除同样触发重拉
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
  });

  it('保存失败：错误提示透出后端 message、不重拉、按钮退出 loading', async () => {
    mSet.mockRejectedValue(new Error('smtp quota exceeded'));
    mFetch.mockResolvedValue({ ...baseSettings, dingtalkUrl: 'https://old' });
    renderTab();
    const input = await screen.findByDisplayValue('https://old');
    fireEvent.change(input, { target: { value: 'https://new' } });
    fireEvent.click(saveButtonOf(input));

    expect(await screen.findByText('smtp quota exceeded')).toBeInTheDocument();
    // finally 分支：savingKey 复位，按钮退出 loading
    await waitFor(() => expect(saveButtonOf(input)).not.toHaveClass('ant-btn-loading'));
    expect(mFetch).toHaveBeenCalledTimes(1);
  });
});

describe('NotificationTab 开关（toggleBool）', () => {
  it('站内信开关关闭：setSiteSetting(key, false) + 「已关闭」', async () => {
    renderTab();
    const [inApp] = await screen.findAllByRole('switch');
    fireEvent.click(inApp);

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('notification.inAppEnabled', false));
    expect(await screen.findByText('已关闭')).toBeInTheDocument();
    // 重拉后 settings.inAppEnabled 仍为 true：开关回到选中态
    expect(screen.getAllByRole('switch')[0]).toBeChecked();
  });

  it('站内信开关开启：起点 false → setSiteSetting(key, true) + 「已开启」双翼', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, inAppEnabled: false });
    renderTab();
    const [inApp] = await screen.findAllByRole('switch');
    expect(inApp).not.toBeChecked();
    fireEvent.click(inApp);

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('notification.inAppEnabled', true));
    expect(await screen.findByText('已开启')).toBeInTheDocument();
    // 重拉后 settings.inAppEnabled 仍为 false：开关回到未选中态
    expect(screen.getAllByRole('switch')[0]).not.toBeChecked();
  });

  it('开关失败：提示「操作失败」兜底链路，不重拉', async () => {
    mSet.mockRejectedValue(undefined); // 无可提取信息：锁定「操作失败」fallback
    renderTab();
    const [inApp] = await screen.findAllByRole('switch');
    fireEvent.click(inApp);

    expect(await screen.findByText('操作失败')).toBeInTheDocument();
    expect(mFetch).toHaveBeenCalledTimes(1);
  });
});

describe('NotificationTab 渲染分支', () => {
  it('placeholderMsg 经 intl 解析（飞书加签/HMAC 密钥），help 文本按字段定义渲染', async () => {
    renderTab();
    expect(await screen.findByPlaceholderText('签名校验密钥')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('HMAC-SHA256 密钥')).toBeInTheDocument();
    expect(screen.getByText('钉钉群 → 群设置 → 机器人 → 添加"自定义"机器人')).toBeInTheDocument();
  });

  it('Card loading 结束后才渲染表单（loading→false finally 分支）', async () => {
    let resolveFetch: (v: NotificationSettings) => void = () => {};
    mFetch.mockImplementation(
      () =>
        new Promise<NotificationSettings>((res) => {
          resolveFetch = res;
        }),
    );
    renderTab();
    // 加载中：Card 骨架，表单未挂载
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    resolveFetch({ ...baseSettings });
    const [inApp] = await screen.findAllByRole('switch');
    expect(inApp).toBeChecked(); // inAppEnabled ?? true
  });
});
