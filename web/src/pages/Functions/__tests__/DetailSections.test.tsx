/**
 * DetailSections 的 PermissionsTab：
 * roles 下拉选项必须来自角色 API（listRoles），加载失败降级为 tags 自由输入；
 * gameId/env 是后端 DTO 不存在的死字段，表单不得再渲染（§12.7 断链，见
 * docs/architecture/game-environment-scope.md）。
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, configure } from '@testing-library/react';
import { Form } from 'antd';
import { PermissionsTab } from '../DetailSections';
import { listRoles } from '@/services/api/permissions';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@/services/api/permissions', () => ({
  listRoles: jest.fn(),
}));
const mockListRoles = jest.mocked(listRoles);

function Harness() {
  const [form] = Form.useForm();
  React.useEffect(() => {
    // 与 useFunctionDetailPage 一致：加载后至少灌入一条默认规则行
    form.setFieldsValue({ items: [{ resource: 'function', actions: ['invoke'], roles: [] }] });
  }, [form]);
  return (
    <PermissionsTab
      functionId="inventory.consume"
      permError=""
      permLoading={false}
      permSaving={false}
      permForm={form}
      onSave={jest.fn().mockResolvedValue(undefined)}
    />
  );
}

describe('PermissionsTab', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('roles 下拉从角色 API 拉取选项', async () => {
    mockListRoles.mockResolvedValue({
      items: [
        { id: 1, name: 'ops' } as { id: number; name: string },
        { id: 2, name: 'game-master' } as { id: number; name: string },
      ],
      total: 2,
      page: 1,
      pageSize: 200,
    });
    render(<Harness />);
    await waitFor(() => expect(mockListRoles).toHaveBeenCalledWith({ pageSize: 200 }));

    // roles 是表单中最后一个 Select（actions 在前），展开后应出现角色选项
    const combos = await screen.findAllByRole('combobox');
    expect(combos.length).toBe(2); // actions + roles
    const rolesRoot = combos[combos.length - 1].closest('.ant-select') as HTMLElement;
    expect(rolesRoot).not.toBeNull();
    fireEvent.mouseDown(rolesRoot);
    fireEvent.click(rolesRoot);
    // antd v6 的 option 是双层节点（role=option 外层 + content 内层），用 AllByText
    const opts = await screen.findAllByText('game-master');
    expect(opts.length).toBeGreaterThan(0);
  });

  it('角色 API 失败时降级：不渲染 gameId/env 死字段，权限表单仍可用', async () => {
    mockListRoles.mockRejectedValue(new Error('boom'));
    render(<Harness />);
    await waitFor(() => expect(mockListRoles).toHaveBeenCalled());
    // 断链字段已移除：不应再出现 gameId / env 输入框
    expect(screen.queryByText('gameId')).not.toBeInTheDocument();
    expect(screen.queryByText('env')).not.toBeInTheDocument();
    // 保存按钮仍存在（表单可用）
    expect(await screen.findByText('保存权限')).toBeInTheDocument();
  });
});
