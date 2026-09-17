import { request } from '@umijs/max';
import {
  drainNode,
  getNodeCommands,
  getNodeMeta,
  listNodes,
  restartNode,
  undrainNode,
  updateNodeMeta,
} from './nodes';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// Authorization 由 requestErrorConfig 的请求拦截器统一注入
// （含无 token 时的省略），适配层只负责 URL/method/参数形状。
describe('nodes API adapters', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
  });

  it('lists nodes with filter params', async () => {
    mockedRequest.mockResolvedValue({ items: [] });

    await listNodes({ type: 'agent', status: 'online' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes', {
      method: 'GET',
      params: { type: 'agent', status: 'online' },
    });
  });

  it('lists nodes without params when none given', async () => {
    mockedRequest.mockResolvedValue({ items: [] });

    await listNodes();

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes', {
      method: 'GET',
      params: undefined,
    });
  });

  it('reads node metadata via GET', async () => {
    mockedRequest.mockResolvedValue({ meta: { region: 'cn-1' } });

    await getNodeMeta('node-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/meta', {
      method: 'GET',
    });
  });

  it('writes node metadata via PUT with a wrapped body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await updateNodeMeta('node-1', { zone: 'z-a' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/meta', {
      method: 'PUT',
      data: { meta: { zone: 'z-a' } },
    });
  });

  it('drains a node with the optional timeout in the body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await drainNode('node-1', 30);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/drain', {
      method: 'POST',
      data: { timeout: 30 },
    });
  });

  it('drains a node without a timeout', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await drainNode('node-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/drain', {
      method: 'POST',
      data: { timeout: undefined },
    });
  });

  it('undrains a node via POST', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await undrainNode('node-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/undrain', {
      method: 'POST',
    });
  });

  it('restarts a node via POST', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await restartNode('node-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/restart', {
      method: 'POST',
    });
  });

  it('lists node commands via GET', async () => {
    mockedRequest.mockResolvedValue({ items: [{ name: 'restart', description: '重启节点' }] });
    await getNodeCommands();

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/commands', {
      method: 'GET',
    });
  });

  it('适配层不注入 Authorization/headers 键（拦截器职责）', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await listNodes();
    await restartNode('node-2');

    for (const call of mockedRequest.mock.calls) {
      expect(call[1]).not.toHaveProperty('headers');
    }
  });
});
