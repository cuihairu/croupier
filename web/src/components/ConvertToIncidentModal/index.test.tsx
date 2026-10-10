/**
 * ConvertToIncidentModal 单测（docs/design/incident-reports.md §4.3 转事故）：
 * - prefill=null → 弹窗不渲染；
 * - 打开即拉启用类别，prefill 标题/严重度进表单，Ref 关联只读回显；
 * - 提交：createIncident 收到表单值 + prefill 的 detectedAt/refType/refId，
 *   成功后回调 onConverted/onClose；
 * - 关联行：refType 缺省时不渲染关联回显。
 *
 * mock 口径：incident 服务全 mock；Modal 是 portal——按 document 查询
 * （web jest 坑档：Modal portal 用 document 查询）；antd 默认 locale=en，
 * 确定按钮文案 OK。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import ConvertToIncidentModal, { type ConvertPrefill } from './index';
import { createIncident, fetchIncidentCategories } from '@/services/api/incident';
import type { IncidentCategory } from '@/services/api/incident';

jest.mock('@/services/api/incident', () => ({
  createIncident: jest.fn(),
  fetchIncidentCategories: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  __esModule: true,
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage || '',
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage || '',
}));

const mCreate = createIncident as jest.MockedFunction<typeof createIncident>;
const mFetch = fetchIncidentCategories as jest.MockedFunction<typeof fetchIncidentCategories>;

const CATS: IncidentCategory[] = [
  {
    id: 1,
    name: '可用性',
    slug: 'availability',
    sort: 1,
    leader: 'alice',
    subcategories: [],
    enabled: true,
    builtin: true,
    createdAt: '2026-10-01T00:00:00Z',
    updatedAt: '2026-10-01T00:00:00Z',
  },
  {
    id: 2,
    name: '运维',
    slug: 'ops',
    sort: 2,
    leader: 'bob',
    subcategories: ['编译', '探活'],
    enabled: true,
    builtin: true,
    createdAt: '2026-10-01T00:00:00Z',
    updatedAt: '2026-10-01T00:00:00Z',
  },
];

const PREFILL: ConvertPrefill = {
  title: 'HighErrorRate on game-svc',
  severity: 'critical',
  detectedAt: '2026-10-10T02:00:00Z',
  refType: 'alert',
  refId: 'HighErrorRate:10.0.0.1',
};

const getOkButton = (): HTMLElement => {
  const btns = Array.from(
    document.querySelectorAll<HTMLButtonElement>('.ant-modal .ant-btn-primary'),
  );
  const ok = btns.find((b) => /OK|确\s*定/.test(b.textContent || ''));
  if (!ok) throw new Error('OK button not found');
  return ok;
};

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue({ items: CATS, total: CATS.length });
  mCreate.mockResolvedValue({ ...CATS[0] } as never);
});

describe('ConvertToIncidentModal', () => {
  it('prefill=null → 不渲染弹窗', () => {
    render(<ConvertToIncidentModal prefill={null} onClose={jest.fn()} />);
    expect(document.querySelector('.ant-modal')).toBeNull();
    expect(mFetch).not.toHaveBeenCalled();
  });

  it('打开即拉类别 + prefill 进表单 + Ref 只读回显', async () => {
    render(<ConvertToIncidentModal prefill={PREFILL} onClose={jest.fn()} />);
    await waitFor(() => expect(mFetch).toHaveBeenCalledWith(true));
    const titleInput = document.querySelector<HTMLInputElement>('.ant-modal input#title');
    expect(titleInput?.value).toBe('HighErrorRate on game-svc');
    // Ref 关联是只读 Input 回显（disabled，排除 Select 的内部 input）
    await waitFor(() => {
      const disabled = Array.from(
        document.querySelectorAll<HTMLInputElement>('.ant-modal input.ant-input[disabled]'),
      ).map((i) => i.value);
      expect(disabled).toContain('alert:HighErrorRate:10.0.0.1');
    });
  });

  it('提交：createIncident 收 prefill 时刻与 Ref，成功后回调', async () => {
    const onClose = jest.fn();
    const onConverted = jest.fn();
    render(
      <ConvertToIncidentModal
        prefill={{ ...PREFILL, categoryId: 2 }}
        onClose={onClose}
        onConverted={onConverted}
      />,
    );
    await waitFor(() => expect(mFetch).toHaveBeenCalled());
    fireEvent.click(getOkButton());
    await waitFor(() => expect(mCreate).toHaveBeenCalled());
    expect(mCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'HighErrorRate on game-svc',
        severity: 'critical',
        categoryId: 2,
        responsibleType: 'unknown',
        detectedAt: '2026-10-10T02:00:00Z',
        refType: 'alert',
        refId: 'HighErrorRate:10.0.0.1',
      }),
    );
    await waitFor(() => expect(onConverted).toHaveBeenCalled());
    expect(onClose).toHaveBeenCalled();
  });

  it('无 refType → 不渲染关联回显行', async () => {
    render(
      <ConvertToIncidentModal
        prefill={{ title: 'x', detectedAt: '2026-10-10T02:00:00Z' }}
        onClose={jest.fn()}
      />,
    );
    await waitFor(() => expect(mFetch).toHaveBeenCalled());
    // 时刻行回显在，Ref 行不在（ant-input 的 disabled 输入只有时刻一个）
    await waitFor(() => {
      const disabled = Array.from(
        document.querySelectorAll<HTMLInputElement>('.ant-modal input.ant-input[disabled]'),
      );
      expect(disabled.length).toBe(1);
      expect(disabled[0].value).toContain('2026-10-10');
    });
  });
});
