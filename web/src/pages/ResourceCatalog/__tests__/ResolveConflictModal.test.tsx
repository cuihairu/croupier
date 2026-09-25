/** ResolveConflictModal：候选值 Tag（displaySemanticValue JSON 解析）、
 * 采用来源下拉只列有值来源、确认/取消回调、conflict 缺省时只出弹窗外壳。 */
import React from 'react';
import { Form } from 'antd';
import { fireEvent, render, screen } from '@testing-library/react';
import ResolveConflictModal from '../ResolveConflictModal';
import type { ResolveSemanticConflictRequest, SemanticConflictInfo } from '@/types/dashboard';

jest.mock('@umijs/max', () => {
  const formatMessage = (
    { defaultMessage }: { defaultMessage: string },
    values?: Record<string, unknown>,
  ) =>
    Object.entries(values || {}).reduce(
      (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
      defaultMessage,
    );
  const intl = { formatMessage, locale: 'zh-CN' };
  return {
    __esModule: true,
    useIntl: () => intl,
    getIntl: () => intl,
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    history: { push: jest.fn() },
  };
});

const CONFLICT: SemanticConflictInfo = {
  field: 'identityField',
  values: {
    platform_review: '"uid"',
    sdk_explicit: 'player_id',
    // openapi_rest 故意缺省：候选 Tag 与下拉选项都不应出现
  },
};

function Harness(props: {
  open?: boolean;
  conflict: SemanticConflictInfo | null;
  onOk: () => void;
  onCancel: () => void;
}) {
  const [form] = Form.useForm<ResolveSemanticConflictRequest>();
  return (
    <ResolveConflictModal
      open={props.open ?? true}
      form={form}
      conflict={props.conflict}
      onOk={props.onOk}
      onCancel={props.onCancel}
    />
  );
}

describe('ResolveConflictModal', () => {
  it('open=false：不渲染弹窗内容', () => {
    render(<Harness open={false} conflict={CONFLICT} onOk={jest.fn()} onCancel={jest.fn()} />);
    expect(screen.queryByText('解决语义冲突')).not.toBeInTheDocument();
  });

  it('conflict 为 null：弹窗外壳在但无表单与候选值', () => {
    render(<Harness conflict={null} onOk={jest.fn()} onCancel={jest.fn()} />);
    expect(screen.getByText('解决语义冲突')).toBeInTheDocument();
    expect(screen.queryByText('采用来源')).not.toBeInTheDocument();
    expect(screen.queryByText('决议原因')).not.toBeInTheDocument();
  });

  it('候选值 Tag：展示字段名与各来源值（JSON 字符串被解析展示）', () => {
    render(<Harness conflict={CONFLICT} onOk={jest.fn()} onCancel={jest.fn()} />);
    expect(screen.getByText('字段')).toBeInTheDocument();
    expect(screen.getByText('identityField')).toBeInTheDocument();
    // displaySemanticValue：'"uid"' JSON.parse 后取字符串值
    expect(screen.getByText(/平台确认:\s*uid/)).toBeInTheDocument();
    expect(screen.getByText(/SDK 显式:\s*player_id/)).toBeInTheDocument();
    // 缺省来源不出 Tag
    expect(screen.queryByText(/OpenAPI REST/)).not.toBeInTheDocument();
  });

  it('采用来源下拉：选项只含存在值的来源', () => {
    render(<Harness conflict={CONFLICT} onOk={jest.fn()} onCancel={jest.fn()} />);
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    expect(screen.getByRole('option', { name: '平台确认' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'SDK 显式' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'OpenAPI REST' })).not.toBeInTheDocument();
  });

  it('确认/取消按钮触发回调；决议原因可输入', () => {
    const onOk = jest.fn();
    const onCancel = jest.fn();
    render(<Harness conflict={CONFLICT} onOk={onOk} onCancel={onCancel} />);

    fireEvent.click(screen.getByRole('button', { name: /确\s*认\s*选\s*择|OK/ }));
    expect(onOk).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /取\s*消|Cancel/ }));
    expect(onCancel).toHaveBeenCalledTimes(1);

    const reason = screen.getByPlaceholderText('说明为什么采用该来源');
    fireEvent.change(reason, { target: { value: '以平台确认为准' } });
    expect(reason).toHaveValue('以平台确认为准');
  });
});
