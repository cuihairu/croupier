/**
 * HTTPS 证书监控页单测（覆盖率巡检：Ops/Certificates/index.tsx 460 行 0% →
 * 收口，零测试页排行第五）。
 *
 * 锁定契约：
 * - 首拉载荷 {page:1, size:10, status:''}；渲染矩阵：域名 `domain:port`
 *   双臂（port 缺省回 443）、颁发者/主体原文、validFrom/validTo/lastChecked
 *   走 formatDateTime（zh-CN toLocaleString）与缺省 '' 双臂；
 * - 剩余天数 Tag 三态（expired/负数红、expiring/≤30 金、其余绿；数值
 *   缺省 '- 天'）与状态 Tag 四色（expired 红/expiring 金/valid 绿/其余
 *   default）；
 * - getStatus 派生链：status 原样（小写化防御臂）/ 缺省按 daysLeft 派生
 *   （<0 expired、≤alertDays expiring、其余 valid）/ 无 daysLeft →
 *   pending；
 * - 状态筛选下拉：选值 → {page:1, status:选中} 重拉；clear → '' 复位；
 * - 分页：total>size 翻第 2 页 → {page:2} 载荷且稳定不回弹（回归锁定：
 *   原 effect 依赖含 page 的 load 身份，翻页即被拉回第 1 页——本轮修定）；
 * - 刷新按钮按当前页重拉；
 * - 重新检查：checkCertificate(id) → 已触发重新检查 + 重拉；失败
 *   「操作失败」；
 * - 检查全部：checkAllCertificates() → 已触发全量检查 + 重拉；失败
 *   「操作失败」；
 * - 移除监听：confirm 取消 → 不触达；确认 → deleteCertificate(id) →
 *   已移除 + 重拉；失败「移除失败」；
 * - 新增域名弹窗：ModalForm 默认值 port 443 / alertDays 30、domain
 *   required 拦截 → addCertificate 载荷 → 已添加 + 回第 1 页重拉 + 关闭；
 *   失败「添加失败」弹窗保持；
 * - load 失败：listCertificates reject →「加载失败」。
 *
 * mock 口径：services/api/ops 五函数 jest.mock；@umijs/max 本地 mock
 * （defaultMessage 即文案 + {days} 内插）；window.confirm spy；antd/
 * pro-components/真实 formatDateTime。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - formatDateTime 的 try/catch catch 臂：new Date 对非法字符串不抛
 *   （返回 Invalid Date），jsdom toLocaleString 不抛；
 * - 分页 onChange 的 `s || size` 右臂：antd Table onChange 恒传数值
 *   pageSize，永非 falsy。
 *
 * 坑实证（antd6 沿用）：ModalForm 提交锚 .ant-modal-footer
 * .ant-btn-primary（确定）；工具栏 Select 在 .ant-card-extra，mouseDown
 * 根 + 点可见 option content，clear 走 mouseEnter + .ant-select-clear；
 * 移除按钮双字中文自动插空格（移 除监听？——「移除监听」四字不插，无碍）；
 * Table 翻页锚 .ant-pagination-item-2。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import OpsCertificatesPage from '../index';
import { formatDateTime } from '@/utils/format';
import type { Certificate } from '@/services/api/ops';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  listCertificates: jest.fn(),
  addCertificate: jest.fn(),
  checkCertificate: jest.fn(),
  checkAllCertificates: jest.fn(),
  deleteCertificate: jest.fn(),
}));

// defaultMessage 即文案 + {days} 内插
const mockIntl = {
  formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string | number>) => {
    let msg = opts.defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) {
        msg = msg.split(`{${k}}`).join(String(v));
      }
    }
    return msg;
  },
};
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
}));

import {
  listCertificates,
  addCertificate,
  checkCertificate,
  checkAllCertificates,
  deleteCertificate,
} from '@/services/api/ops';

const mList = listCertificates as jest.MockedFunction<typeof listCertificates>;
const mAdd = addCertificate as jest.MockedFunction<typeof addCertificate>;
const mCheck = checkCertificate as jest.MockedFunction<typeof checkCertificate>;
const mCheckAll = checkAllCertificates as jest.MockedFunction<typeof checkAllCertificates>;
const mDelete = deleteCertificate as jest.MockedFunction<typeof deleteCertificate>;

const mk = (over: Partial<Certificate> & Pick<Certificate, 'id' | 'domain'>): Certificate => ({
  port: 443,
  ...over,
});

const T1 = '2026-08-01T08:00:00Z';
const T2 = '2026-09-10T08:00:00Z';
const T3 = '2026-09-20T09:30:00Z';

// 覆盖翼：valid + 满字段（日期/颁发者/主体）
const c1 = mk({
  id: 1,
  domain: 'a.com',
  status: 'valid',
  daysLeft: 90,
  issuer: 'Leto CA',
  subject: 'a.com',
  validFrom: T1,
  validTo: T2,
  lastChecked: T3,
});
// 覆盖翼：expiring 金档（st 命中）
const c2 = mk({ id: 2, domain: 'b.com', port: 8443, status: 'expiring', daysLeft: 15 });
// 覆盖翼：expired 红（st 命中）+ 负数天数 '-5 天'
const c3 = mk({ id: 3, domain: 'c.com', status: 'expired', daysLeft: -5 });
// 覆盖翼：status 缺省 + daysLeft ≤ alertDays → 派生 expiring + 金档（v 命中）
const c4 = mk({ id: 4, domain: 'd.com', daysLeft: 10, alertDays: 30 });
// 覆盖翼：status 缺省 + 负数 → 派生 expired + 红档（v 命中）
const c5 = mk({ id: 5, domain: 'e.com', daysLeft: -1 });
// 覆盖翼：status 缺省 + 大天数 → 派生 valid + 绿档
const c6 = mk({ id: 6, domain: 'f.com', daysLeft: 200 });
// 覆盖翼：status/daysLeft 双缺省 → pending + '- 天' 绿档
const c7 = mk({ id: 7, domain: 'g.com' });
// 覆盖翼：大写 status → toLowerCase 防御臂（状态列小写化；天数列按原文不命中金档）
const c8 = mk({ id: 8, domain: 'h.com', status: 'EXPIRING' as never, daysLeft: 100 });
// 覆盖翼：port 缺省 → ':443'；errorMessage → 错误红 Tag + Tooltip；日期缺省 ''
const c9 = mk({
  id: 9,
  domain: 'i.com',
  port: undefined as never,
  errorMessage: 'handshake failed',
});
// 覆盖翼：expiring 态 + daysLeft 缺省 → 金档 `${v ?? ''} 天` null 臂
const c10 = mk({ id: 10, domain: 'j.com', status: 'expiring' });
// 覆盖翼：expired 态 + daysLeft 缺省 → 红档 `${v != null ? v : '-'}` null 臂
const c11 = mk({ id: 11, domain: 'k.com', status: 'expired' });

const certs = [c1, c2, c3, c4, c5, c6, c7, c8, c9, c10];

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ certificates: certs, total: certs.length, page: 1, size: 10 });
  mAdd.mockResolvedValue(undefined);
  mCheck.mockResolvedValue(undefined);
  mCheckAll.mockResolvedValue(undefined);
  mDelete.mockResolvedValue(undefined);
});

function renderPage() {
  return render(
    <App>
      <OpsCertificatesPage />
    </App>,
  );
}

/** 等首拉落定 */
async function waitLoad() {
  expect(await screen.findByText('a.com:443')).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledWith({ page: 1, size: 10, status: '' }));
}

/** 表格行（锚 domain:port 文本） */
function rowOf(cell: string) {
  return within(document.querySelector('.ant-table') as HTMLElement)
    .getByText(cell)
    .closest('tr') as HTMLElement;
}

/** 工具栏状态 Select 选 option */
async function pickStatusFilter(label: string) {
  await new Promise((r) => setTimeout(r, 60));
  const select = document.querySelector('.ant-card-extra .ant-select') as HTMLElement;
  fireEvent.mouseDown(select);
  const dropdown = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
      (d) => !d.className.includes('ant-select-dropdown-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  fireEvent.click(
    within(dropdown).getByText(label, { selector: '.ant-select-item-option-content' }),
  );
}

/** 弹窗 footer 确定按钮 */
function submitForm() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

describe('证书监控 初始渲染', () => {
  it('矩阵：域名 port 回退 / 日期三列 / 剩余天数三态 / 状态四色 / 派生链 / 错误 Tag', async () => {
    renderPage();
    await waitLoad();

    // 域名双臂 + 颁发者/主体 + 日期（与 formatDateTime 同口径）
    expect(within(rowOf('b.com:8443')).getByText('b.com:8443')).toBeInTheDocument();
    expect(within(rowOf('i.com:443')).getByText('i.com:443')).toBeInTheDocument();
    expect(within(rowOf('a.com:443')).getByText('Leto CA')).toBeInTheDocument();
    expect(within(rowOf('a.com:443')).getByText('a.com')).toBeInTheDocument();
    const fmtT1 = formatDateTime(T1);
    expect(within(rowOf('a.com:443')).getByText(fmtT1)).toBeInTheDocument();

    // 剩余天数三态：红（st/负数）、金（st/≤30）、绿（其余）、'- 天' 缺省
    expect(within(rowOf('c.com:443')).getByText('-5 天').closest('.ant-tag')).toHaveClass(
      'ant-tag-red',
    );
    expect(within(rowOf('e.com:443')).getByText('-1 天').closest('.ant-tag')).toHaveClass(
      'ant-tag-red',
    );
    expect(within(rowOf('b.com:8443')).getByText('15 天').closest('.ant-tag')).toHaveClass(
      'ant-tag-gold',
    );
    expect(within(rowOf('d.com:443')).getByText('10 天').closest('.ant-tag')).toHaveClass(
      'ant-tag-gold',
    );
    expect(within(rowOf('a.com:443')).getByText('90 天').closest('.ant-tag')).toHaveClass(
      'ant-tag-green',
    );
    expect(within(rowOf('f.com:443')).getByText('200 天').closest('.ant-tag')).toHaveClass(
      'ant-tag-green',
    );
    expect(within(rowOf('g.com:443')).getByText('- 天').closest('.ant-tag')).toHaveClass(
      'ant-tag-green',
    );

    // 状态四色 + 派生链 + 大写防御臂
    expect(within(rowOf('a.com:443')).getByText('valid').closest('.ant-tag')).toHaveClass(
      'ant-tag-green',
    );
    expect(within(rowOf('b.com:8443')).getByText('expiring').closest('.ant-tag')).toHaveClass(
      'ant-tag-gold',
    );
    expect(within(rowOf('c.com:443')).getByText('expired').closest('.ant-tag')).toHaveClass(
      'ant-tag-red',
    );
    expect(within(rowOf('g.com:443')).getByText('pending').closest('.ant-tag')).not.toHaveClass(
      'ant-tag-gold',
    );
    // 大写 EXPIRING → 状态列小写化 gold；天数列拿原文不命中金档（100 → 绿）
    expect(within(rowOf('h.com:443')).getByText('expiring').closest('.ant-tag')).toHaveClass(
      'ant-tag-gold',
    );
    expect(within(rowOf('h.com:443')).getByText('100 天').closest('.ant-tag')).toHaveClass(
      'ant-tag-green',
    );

    // errorMessage → 红「错误」Tag + Tooltip title
    const errTag = within(rowOf('i.com:443')).getByText('错误').closest('.ant-tag');
    expect(errTag).toHaveClass('ant-tag-red');

    // expiring + daysLeft 缺省 → 金档空串天数
    expect(within(rowOf('j.com:443')).getByText('天').closest('.ant-tag')).toHaveClass(
      'ant-tag-gold',
    );
  });

  it('天数缺省双臂：expired → 红「- 天」（分页内放不下，独立渲染）', async () => {
    mList.mockResolvedValueOnce({ certificates: [c11], total: 1, page: 1, size: 10 });
    renderPage();
    expect(await screen.findByText('k.com:443')).toBeInTheDocument();
    expect(within(rowOf('k.com:443')).getByText('- 天').closest('.ant-tag')).toHaveClass(
      'ant-tag-red',
    );
    expect(within(rowOf('k.com:443')).getByText('expired').closest('.ant-tag')).toHaveClass(
      'ant-tag-red',
    );
  });

  it('load 失败：reject → 加载失败；响应缺省四右臂 → 空表', async () => {
    mList.mockRejectedValueOnce(new Error('cert-down'));
    renderPage();
    expect(await screen.findByText('加载失败')).toBeInTheDocument();

    // 响应缺省（certificates/total/page/size 四 || 右臂）→ 空表不炸
    mList.mockResolvedValueOnce({} as never);
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-table .ant-empty-description')).not.toBeNull(),
    );
  });
});

describe('证书监控 筛选与分页', () => {
  it('状态筛选选值重拉第 1 页；clear 复位', async () => {
    renderPage();
    await waitLoad();

    await pickStatusFilter('expired');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({ page: 1, size: 10, status: 'expired' }),
    );

    // clear → onChange(undefined) → '' 复位
    const select = document.querySelector('.ant-card-extra .ant-select') as HTMLElement;
    fireEvent.mouseEnter(select);
    fireEvent.click(select.querySelector('.ant-select-clear') as HTMLElement);
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({ page: 1, size: 10, status: '' }));
  });

  it('分页翻第 2 页：{page:2} 载荷且稳定不回弹', async () => {
    // 响应回显请求页（服务端契约），否则 mount 即被顶到第 2 页、点击落空
    mList.mockImplementation(async (p?: { page?: number; size?: number; status?: string }) => ({
      certificates: certs,
      total: 25,
      page: p?.page || 1,
      size: p?.size || 10,
    }));
    renderPage();
    await waitLoad();

    fireEvent.click(document.querySelector('.ant-pagination-item-2') as HTMLElement);
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({ page: 2, size: 10, status: '' }));
    // 回归锁定：等一拍确认没有 effect 把页码拉回 1
    await new Promise((r) => setTimeout(r, 150));
    expect(mList).toHaveBeenLastCalledWith({ page: 2, size: 10, status: '' });
  });

  it('刷新按当前页重拉', async () => {
    renderPage();
    await waitLoad();
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    expect(mList).toHaveBeenLastCalledWith({ page: 1, size: 10, status: '' });
  });
});

describe('证书监控 操作', () => {
  it('重新检查：checkCertificate(id) → 已触发重新检查 + 重拉；失败「操作失败」', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('a.com:443')).getByRole('button', { name: /重新检查/ }));
    await waitFor(() => expect(mCheck).toHaveBeenCalledWith(1));
    expect(await screen.findByText('已触发重新检查')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    mCheck.mockRejectedValueOnce(new Error('chk-x'));
    fireEvent.click(within(rowOf('a.com:443')).getByRole('button', { name: /重新检查/ }));
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);
  });

  it('检查全部：checkAllCertificates → 已触发全量检查 + 重拉；失败「操作失败」', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /检查全部/ }));
    await waitFor(() => expect(mCheckAll).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('已触发全量检查')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    mCheckAll.mockRejectedValueOnce(new Error('all-x'));
    fireEvent.click(screen.getByRole('button', { name: /检查全部/ }));
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);
  });

  it('移除监听：confirm 取消不触达；确认 → deleteCertificate → 已移除 + 重拉；失败「移除失败」', async () => {
    const confirmSpy = jest.spyOn(window, 'confirm').mockReturnValue(false);
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('b.com:8443')).getByRole('button', { name: /移除监听/ }));
    await waitFor(() => expect(confirmSpy).toHaveBeenCalledTimes(1));
    expect(mDelete).not.toHaveBeenCalled();

    confirmSpy.mockReturnValue(true);
    fireEvent.click(within(rowOf('b.com:8443')).getByRole('button', { name: /移除监听/ }));
    await waitFor(() => expect(mDelete).toHaveBeenCalledWith(2));
    expect(await screen.findByText('已移除')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    mDelete.mockRejectedValueOnce(new Error('del-x'));
    fireEvent.click(within(rowOf('b.com:8443')).getByRole('button', { name: /移除监听/ }));
    expect(await screen.findByText('移除失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);
  });
});

describe('证书监控 新增域名', () => {
  it('required 拦截 → 默认值载荷（port 443/alertDays 30）→ 已添加 + 回第 1 页 + 关闭', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /新增域名/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('新增域名'),
    );

    // 空提交：domain required
    submitForm();
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mAdd).not.toHaveBeenCalled();

    // 只填域名：port/alertDays 走默认值
    fireEvent.change(screen.getByPlaceholderText('example.com'), {
      target: { value: 'new.example.com' },
    });
    submitForm();
    await waitFor(() => expect(mAdd).toHaveBeenCalledTimes(1));
    expect(mAdd).toHaveBeenCalledWith({
      domain: 'new.example.com',
      port: 443,
      alertDays: 30,
    });
    expect(await screen.findByText('已添加')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    expect(mList).toHaveBeenLastCalledWith({ page: 1, size: 10, status: '' });
    await waitFor(() =>
      expect(screen.queryByPlaceholderText('example.com')).not.toBeInTheDocument(),
    );
  });

  it('添加失败：弹窗保持 +「添加失败」', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /新增域名/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('新增域名'),
    );
    fireEvent.change(screen.getByPlaceholderText('example.com'), {
      target: { value: 'dup.example.com' },
    });

    mAdd.mockRejectedValueOnce(new Error('dup'));
    submitForm();
    expect(await screen.findByText('添加失败')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('example.com')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});
