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
const mockedGetItem = localStorage.getItem as unknown as jest.Mock;

describe('nodes API adapters', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
    mockedGetItem.mockReset().mockReturnValue('tok-nodes');
  });

  it('lists nodes with filter params and bearer token', async () => {
    mockedRequest.mockResolvedValue({ items: [] });

    await listNodes({ type: 'agent', status: 'online' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes', {
      method: 'GET',
      params: { type: 'agent', status: 'online' },
      headers: { Authorization: 'Bearer tok-nodes' },
    });
  });

  it('lists nodes without params when none given', async () => {
    mockedRequest.mockResolvedValue({ items: [] });

    await listNodes();

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes', {
      method: 'GET',
      params: undefined,
      headers: { Authorization: 'Bearer tok-nodes' },
    });
  });

  it('omits the Authorization header when no token is stored', async () => {
    mockedGetItem.mockReturnValue(undefined);
    mockedRequest.mockResolvedValue({ items: [] });

    await listNodes();

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes', {
      method: 'GET',
      params: undefined,
      headers: undefined,
    });
  });

  it('reads node metadata via GET', async () => {
    mockedRequest.mockResolvedValue({ meta: { region: 'cn-1' } });

    await getNodeMeta('node-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/meta', {
      method: 'GET',
      headers: { Authorization: 'Bearer tok-nodes' },
    });
  });

  it('writes node metadata via PUT with a wrapped body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await updateNodeMeta('node-1', { zone: 'z-a' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/meta', {
      method: 'PUT',
      data: { meta: { zone: 'z-a' } },
      headers: { Authorization: 'Bearer tok-nodes' },
    });
  });

  it('drains a node with the optional timeout in the body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await drainNode('node-1', 30);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/drain', {
      method: 'POST',
      data: { timeout: 30 },
      headers: { Authorization: 'Bearer tok-nodes' },
    });
  });

  it('drains a node without a timeout', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await drainNode('node-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/drain', {
      method: 'POST',
      data: { timeout: undefined },
      headers: { Authorization: 'Bearer tok-nodes' },
    });
  });

  it('undrains a node via POST', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await undrainNode('node-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/undrain', {
      method: 'POST',
      headers: { Authorization: 'Bearer tok-nodes' },
    });
  });

  it('restarts a node via POST', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await restartNode('node-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-1/restart', {
      method: 'POST',
      headers: { Authorization: 'Bearer tok-nodes' },
    });
  });

  it('drops the header for mutations too when token is absent', async () => {
    mockedGetItem.mockReturnValue(undefined);
    mockedRequest.mockResolvedValue(undefined);

    await restartNode('node-2');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/node-2/restart', {
      method: 'POST',
      headers: undefined,
    });
  });

  it('lists node commands via GET', async () => {
    mockedRequest.mockResolvedValue({ items: [{ name: 'restart', description: '重启节点' }] });
    await getNodeCommands();

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/commands', {
      method: 'GET',
      headers: { Authorization: 'Bearer tok-nodes' },
    });
  });

  // 不可达分支说明：每个函数内 `typeof window !== 'undefined' ? ... : ''` 的
  // false 路径是 SSR 防御守卫。jsdom 环境中 globalThis.window 为
  // non-configurable（Object.defineProperty 重定义抛 "Cannot redefine
  // property: window"），无法在单测中置为 undefined，故该分支不可达。
});
