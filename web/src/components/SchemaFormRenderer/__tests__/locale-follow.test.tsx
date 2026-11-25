/**
 * SUI-024 验收：表单文案跟随当前语言，且切换中英文不重置已填写数据。
 *
 * 真实 RJSF 引擎 + 真实 antd Input/Select。reload=false 语言切换经 umi
 * plugin-locale 广播的 window 'languagechange' 事件触发派生重算（renderer
 * 订阅该事件）；formData 镜像即实时值，schema/uiSchema 换语言不回滚已填数据。
 * 测试经全局 @umijs/max 替身的 getLocale 可控 locale。
 */
import React from 'react';
import * as umiMax from '@umijs/max';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const umiMock = umiMax as unknown as { getLocale?: () => string };

const spec: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    properties: {
      name: { type: 'string', title: '名称' },
      mode: {
        type: 'string',
        title: '类型',
        enum: ['mail', 'broadcast'],
        'x-enum-options': [
          { value: 'mail', label: { 'zh-CN': '邮件', 'en-US': 'Mail' } },
          { value: 'broadcast', label: { 'zh-CN': '全服广播', 'en-US': 'Broadcast' } },
        ],
      },
    },
  } as unknown as JSONSchema,
};

function switchLocale(getLocale: () => string): void {
  umiMock.getLocale = getLocale;
  act(() => {
    window.dispatchEvent(new Event('languagechange'));
  });
}

describe('SUI-024: 切换语言不重置已填写数据（真实 RJSF 提交路径）', () => {
  afterEach(() => {
    delete umiMock.getLocale;
  });

  it('填值 → 切 en-US：文案换语言、输入值保留、提交值不变', async () => {
    const onFinish = jest.fn();
    const view = render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);

    const nameInput = await screen.findByLabelText('名称');
    await act(async () => {
      fireEvent.change(nameInput, { target: { value: 'hello' } });
    });

    switchLocale((): string => 'en-US');

    // 字段标题换语言（schema title 来自 spec.jsonSchema 静态 zh——这里验证
    // 枚举下拉标签换语言：x-enum-options 是 SUI-024 动态取 locale 的面）
    fireEvent.mouseDown(screen.getByRole('combobox'));
    await waitFor(() => {
      expect(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length).toBeGreaterThan(0);
    });
    const labels = Array.from(
      document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
    )
      .filter((el) => (el.textContent ?? '').length > 0)
      .map((el) => el.textContent);
    expect(labels).toEqual(['Mail', 'Broadcast']);

    // 已填写数据不重置
    expect((screen.getByLabelText('名称') as HTMLInputElement).value).toBe('hello');

    // 枚举点选 + 提交：镜像值完整（name 保留 + mode 原始枚举值）
    await act(async () => {
      const target = Array.from(
        document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
      ).find((el) => el.textContent === 'Broadcast');
      fireEvent.click(target as Element);
    });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交|submit/i }));
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ name: 'hello', mode: 'broadcast' });

    // 切回 zh-CN：标签复原（恢复缺省 locale 后新挂载）
    view.unmount();
    delete umiMock.getLocale;
    const second = render(<SchemaFormRenderer spec={spec} onFinish={jest.fn()} />);
    fireEvent.mouseDown(second.getByRole('combobox'));
    await waitFor(() => {
      expect(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length).toBeGreaterThan(0);
    });
    const zhLabels = Array.from(
      document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
    )
      .filter((el) => (el.textContent ?? '').length > 0)
      .map((el) => el.textContent);
    expect(zhLabels).toEqual(['邮件', '全服广播']);
    second.unmount();
  }, 15000);

  it('挂载后订阅 languagechange：en-US 下挂载直接渲染 en 文案', async () => {
    umiMock.getLocale = (): string => 'en-US';
    const view = render(<SchemaFormRenderer spec={spec} onFinish={jest.fn()} />);
    fireEvent.mouseDown(view.getByRole('combobox'));
    await waitFor(() => {
      expect(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length).toBeGreaterThan(0);
    });
    const labels = Array.from(
      document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
    )
      .filter((el) => (el.textContent ?? '').length > 0)
      .map((el) => el.textContent);
    expect(labels).toEqual(['Mail', 'Broadcast']);
    view.unmount();
  }, 15000);
});
