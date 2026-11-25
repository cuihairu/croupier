/**
 * SUI-025 验收：无配置 = 标准纵向表单（渲染端兜底）。
 *
 * spec 未携带 fields（发布页 Go 生成器对无 hints schema 不产出 fields；调试页
 * derivePresentationSpec 同语义）时，渲染端按 jsonSchema 兜底合成标准字段
 * （含类型缺省控件），与有配置发布页同一渲染管线。真实 RJSF 引擎 +
 * 真实 SchemaFormRenderer，断言文本域/勾选控件落地与提交投影。
 */
import React, { createRef } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const fallbackSpec: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    properties: {
      note: { type: 'string', maxLength: 300 },
      flag: { type: 'boolean' },
    },
  } as unknown as JSONSchema,
  layout: 'vertical',
};

describe('SchemaFormRenderer no-config standard vertical form (SUI-025)', () => {
  test(
    'fields-less spec synthesizes type-default widgets (TextArea/Switch) and submits',
    async () => {
      const onFinish = jest.fn();
      const ref = createRef<SchemaFormRendererHandle>();
      render(<SchemaFormRenderer spec={fallbackSpec} onFinish={onFinish} ref={ref} />);

      // 类型缺省控件落地：maxLength>120 → TextArea（textarea 元素）、boolean →
      // Switch（ui:widget checkbox → checkbox 元素）
      const noteArea = document.querySelector('textarea');
      expect(noteArea).not.toBeNull();
      const checkbox = document.querySelector('input[type="checkbox"]');
      expect(checkbox).not.toBeNull();

      // label 兜底链（F5）：派生字段无 label → schema 无 title → key 人性化
      expect(screen.getByText('Note')).toBeInTheDocument();
      expect(screen.getByText('Flag')).toBeInTheDocument();

      fireEvent.change(noteArea as HTMLTextAreaElement, { target: { value: 'hello' } });
      fireEvent.click(checkbox as HTMLInputElement);
      ref.current?.submit();
      await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
      expect(onFinish).toHaveBeenCalledWith({ note: 'hello', flag: true });
    },
    15000,
  );
});
