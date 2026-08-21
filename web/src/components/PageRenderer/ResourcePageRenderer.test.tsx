import { fireEvent, render, screen } from '@testing-library/react';
import ResourcePageRenderer from './ResourcePageRenderer';
import type { PageFunctionBinding, ResourcePageSpec } from '@/types/dashboard';

const resource: ResourcePageSpec = {
  listView: {
    identityKey: 'id',
    columns: [{ key: 'id', title: { 'zh-CN': 'ID' }, dataType: 'string' }],
    pagination: { enabled: true, defaultSize: 20 },
  },
};

const resourceWithDetail: ResourcePageSpec = {
  ...resource,
  detailView: {
    layout: 'vertical',
    fields: [{ key: 'id', title: { 'zh-CN': 'ID' }, dataType: 'string' }],
  },
};

function listBinding(output: PageFunctionBinding['selectors']['output']): PageFunctionBinding {
  return {
    id: 'list',
    functionId: 'player.list',
    usage: 'query',
    execution: { mode: 'sync' },
    selectors: { input: { assignments: [] }, output },
  };
}

describe('ResourcePageRenderer', () => {
  test('列表结果缺少 items selector 时显示可定位错误', async () => {
    render(
      <ResourcePageRenderer
        spec={resource}
        bindings={[listBinding([])]}
        onExecute={async () => ({ kind: 'sync', requestId: 'request-1', data: { items: [] } })}
      />,
    );

    await screen.findByText('列表绑定缺少 pageState.items 输出 selector，无法渲染查询结果');
  });

  test('列表结果按 items 与 total selector 渲染真实数据', async () => {
    render(
      <ResourcePageRenderer
        spec={resource}
        bindings={[
          listBinding([
            { stateKey: 'items', source: '/payload/records', shape: 'collection' },
            { stateKey: 'total', source: '/payload/total', shape: 'scalar' },
          ]),
        ]}
        onExecute={async () => ({
          kind: 'sync',
          requestId: 'request-1',
          data: { payload: { records: [{ id: 'p-1' }], total: 1 } },
        })}
      />,
    );

    await screen.findByText('p-1');
    expect(screen.getByText('p-1')).toBeInTheDocument();
  });

  test('详情结果缺少 detail selector 时显示可定位错误', async () => {
    render(
      <ResourcePageRenderer
        spec={resourceWithDetail}
        bindings={[
          listBinding([{ stateKey: 'items', source: '/payload/records', shape: 'collection' }]),
          {
            id: 'detail',
            functionId: 'player.detail',
            usage: 'detail',
            execution: { mode: 'sync' },
            selectors: { input: { assignments: [] }, output: [] },
          },
        ]}
        onExecute={async (bindingID) =>
          bindingID === 'list'
            ? {
                kind: 'sync',
                requestId: 'request-list',
                data: { payload: { records: [{ id: 'p-1' }] } },
              }
            : { kind: 'sync', requestId: 'request-detail', data: { payload: { id: 'p-1' } } }
        }
      />,
    );

    await screen.findByText('p-1');
    fireEvent.click(screen.getByRole('button', { name: /查看/ }));

    await screen.findByText('详情绑定缺少 pageState.detail 输出 selector，无法渲染详情结果');
  });

  test('详情结果按 detail selector 渲染真实字段', async () => {
    render(
      <ResourcePageRenderer
        spec={resourceWithDetail}
        bindings={[
          listBinding([{ stateKey: 'items', source: '/payload/records', shape: 'collection' }]),
          {
            id: 'detail',
            functionId: 'player.detail',
            usage: 'detail',
            execution: { mode: 'sync' },
            selectors: {
              input: { assignments: [] },
              output: [{ stateKey: 'detail', source: '/payload/player', shape: 'object' }],
            },
          },
        ]}
        onExecute={async (bindingID) =>
          bindingID === 'list'
            ? {
                kind: 'sync',
                requestId: 'request-list',
                data: { payload: { records: [{ id: 'p-1' }] } },
              }
            : {
                kind: 'sync',
                requestId: 'request-detail',
                data: { payload: { player: { id: 'p-2' } } },
              }
        }
      />,
    );

    await screen.findByText('p-1');
    fireEvent.click(screen.getByRole('button', { name: /查看/ }));

    await screen.findByText('p-2');
  });
});

describe('ResourcePageRenderer action-form row action', () => {
  it('opens a form modal for actions carrying form', async () => {
    const onExecute = jest.fn().mockImplementation((bindingId: string) =>
      bindingId === 'list'
        ? Promise.resolve({
            kind: 'sync' as const,
            requestId: 'r',
            data: { payload: { items: [{ id: 'p-1', name: 'Alice' }], total: 1 } },
          })
        : Promise.resolve({ kind: 'sync' as const, requestId: 'r', data: {} }),
    );
    const spec: ResourcePageSpec = {
      ...resource,
      listView: {
        ...resource.listView,
        columns: [
          { key: 'id', title: { 'zh-CN': 'ID' }, dataType: 'string' },
          { key: 'name', title: { 'zh-CN': 'Name' }, dataType: 'string' },
        ],
        rowActions: [
          {
            key: 'ban',
            title: { 'zh-CN': '封禁' },
            bindingId: 'action.ban',
            form: {
              jsonSchema: {
                type: 'object',
                properties: { reason: { type: 'string', title: 'Reason' } },
                required: ['reason'],
              },
              layout: 'vertical',
            },
          },
        ],
      },
    };
    const bindings: PageFunctionBinding[] = [
      listBinding([
        { stateKey: 'items', source: '/payload/items', shape: 'collection' },
        { stateKey: 'total', source: '/payload/total', shape: 'scalar' },
      ]),
      {
        id: 'action.ban',
        functionId: 'player.ban',
        usage: 'action',
        execution: { mode: 'sync' },
        selectors: {
          input: {
            assignments: [
              { target: '/id', source: { kind: 'row', path: '/id' } },
              { target: '/reason', source: { kind: 'form', path: '/reason' } },
            ],
          },
          output: [],
        },
      },
    ];

    render(<ResourcePageRenderer spec={spec} bindings={bindings} onExecute={onExecute} />);

    // 数据行渲染后按钮才出现
    await screen.findByText('p-1');
    fireEvent.click(screen.getByRole('button', { name: /封禁/ }));
    // 弹出表单操作 Modal，含剥离 identity 后的 reason 字段
    expect(await screen.findByText('Reason')).toBeInTheDocument();
  });
});
