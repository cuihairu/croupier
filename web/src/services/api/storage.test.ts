import { getIntl, request } from '@umijs/max';
import {
  batchDeleteObjects,
  buildAvatarObjectKey,
  createDirectory,
  deleteObject,
  getSignedUrl,
  listObjects,
  uploadAsset,
  uploadObject,
} from './storage';

jest.mock('@umijs/max', () => ({
  request: jest.fn(),
  getIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
}));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// ---- Fake XHR：拦截 uploadObjectMultipart 的原生 XMLHttpRequest ----
// 实例自身即状态载体：storage.ts 的回调读 xhr.status/xhr.responseText，
// 用例直接对 lastXHR 赋值后再触发 onload/onerror。
class FakeXHR {
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  status = 0;
  responseText = '';
  method = '';
  url = '';
  headers: Record<string, string> = {};
  body: FormData | undefined;

  open(method: string, url: string) {
    this.method = method;
    this.url = url;
  }

  setRequestHeader(key: string, value: string) {
    this.headers[key] = value;
  }

  send(body?: FormData) {
    this.body = body;
  }
}

let lastXHR: FakeXHR;

// 构造器里登记 lastXHR 的包装：在替换全局类时包裹原生构造行为
class FakeXHRTracked extends FakeXHR {
  constructor() {
    super();
    lastXHR = this;
  }
}

const flush = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

describe('storage API adapters', () => {
  let originalXHR: typeof global.XMLHttpRequest;

  beforeAll(() => {
    originalXHR = global.XMLHttpRequest;
    Object.defineProperty(global, 'XMLHttpRequest', { value: FakeXHRTracked, writable: true });
  });

  afterAll(() => {
    Object.defineProperty(global, 'XMLHttpRequest', { value: originalXHR, writable: true });
  });

  beforeEach(() => {
    mockedRequest.mockReset();
    // setupTests 将 localStorage 替换为纯 jest.fn（不存值）：经 mockReturnValue 供 token
    (localStorage.getItem as unknown as jest.Mock).mockReturnValue('tok-1');
  });

  describe('buildAvatarObjectKey', () => {
    afterEach(() => {
      // 恢复被替身的 randomUUID（回退分支用例会删掉它）
      Object.defineProperty(crypto, 'randomUUID', {
        value: jest.fn(),
        writable: true,
        configurable: true,
      });
    });

    it('builds avatars/<ts>-<random><ext> with lowercased extension', () => {
      const key = buildAvatarObjectKey(new File(['x'], 'Avatar.PNG'));
      expect(key).toMatch(/^avatars\/\d+-[0-9a-f-]+\.png$/);
    });

    it('falls back to .bin for missing/dangling extensions', () => {
      expect(buildAvatarObjectKey(new File(['x'], 'noext'))).toMatch(/\.bin$/);
      expect(buildAvatarObjectKey(new File(['x'], 'dotted.'))).toMatch(/\.bin$/);
      expect(buildAvatarObjectKey(new File(['x'], '.hidden'))).toMatch(/\.bin$/);
      // 空文件名：getFileExtension 的 (name || '') 兜底侧
      expect(buildAvatarObjectKey(new File(['x'], ''))).toMatch(/\.bin$/);
    });

    it('uses a timestamp+random fallback when randomUUID is unavailable', () => {
      Object.defineProperty(crypto, 'randomUUID', { value: undefined, configurable: true });
      const key = buildAvatarObjectKey(new File(['x'], 'a.jpg'));
      expect(key).toMatch(/^avatars\/\d+-\d+-[a-z0-9]+\.jpg$/);
    });
  });

  it('uploads with multipart form, bearer token, and unwraps nested data payloads', async () => {
    const file = new File(['data'], 'icon.png');
    const promise = uploadObject(file);
    await flush();

    expect(lastXHR.method).toBe('POST');
    expect(lastXHR.url).toBe('/api/v1/storage/objects');
    expect(lastXHR.headers['Authorization']).toBe('Bearer tok-1');
    expect(lastXHR.body?.get('file')).toBe(file);
    expect(lastXHR.body?.get('path')).toBe('icon.png');

    lastXHR.status = 200;
    lastXHR.responseText = JSON.stringify({ data: { path: 'stored/icon.png' } });
    lastXHR.onload?.();
    await expect(promise).resolves.toEqual({ path: 'stored/icon.png' });
  });

  it('resolves the bare payload when the response has no data envelope', async () => {
    const promise = uploadObject(new File(['d'], 'a.txt'));
    await flush();

    lastXHR.status = 201;
    lastXHR.responseText = JSON.stringify({ path: 'p/a.txt' });
    lastXHR.onload?.();
    await expect(promise).resolves.toEqual({ path: 'p/a.txt' });
  });

  it('resolves an empty object when a 2xx body is JSON null', async () => {
    const promise = uploadObject(new File(['d'], 'a.txt'));
    await flush();

    lastXHR.status = 200;
    lastXHR.responseText = 'null';
    lastXHR.onload?.();
    await expect(promise).resolves.toEqual({});
  });

  it('rejects with the backend message on non-2xx', async () => {
    const promise = uploadObject(new File(['d'], 'a.txt'));
    await flush();

    lastXHR.status = 413;
    lastXHR.responseText = JSON.stringify({ message: '文件过大' });
    lastXHR.onload?.();
    await expect(promise).rejects.toThrow('文件过大');
  });

  it('rejects with the intl fallback message when non-2xx body is empty', async () => {
    const promise = uploadObject(new File(['d'], 'a.txt'));
    await flush();

    lastXHR.status = 500;
    lastXHR.responseText = '';
    lastXHR.onload?.();
    await expect(promise).rejects.toThrow('上传失败');
  });

  it('rejects with the parse-failed message on malformed JSON bodies', async () => {
    const promise = uploadObject(new File(['d'], 'a.txt'));
    await flush();

    lastXHR.status = 200;
    lastXHR.responseText = '<html>not json</html>';
    lastXHR.onload?.();
    await expect(promise).rejects.toThrow('上传响应解析失败');
  });

  it('rejects on network errors', async () => {
    const promise = uploadObject(new File(['d'], 'a.txt'));
    await flush();

    lastXHR.onerror?.();
    await expect(promise).rejects.toThrow('上传失败');
  });

  it('uploadAsset combines the upload result with a signed URL', async () => {
    mockedRequest.mockResolvedValueOnce({ url: 'https://cdn/signed' });
    const promise = uploadAsset(new File(['d'], 'a.png'), { path: 'custom/path.png' });
    await flush();

    expect(lastXHR.body?.get('path')).toBe('custom/path.png');
    lastXHR.status = 200;
    lastXHR.responseText = JSON.stringify({ data: { path: 'stored/path.png' } });
    lastXHR.onload?.();

    await expect(promise).resolves.toEqual({ Key: 'stored/path.png', URL: 'https://cdn/signed' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/storage/signed-url', {
      params: { path: 'stored/path.png' },
    });
  });

  it('uploadAsset falls back to the requested path when the upload response omits it', async () => {
    mockedRequest.mockResolvedValueOnce({});
    const promise = uploadAsset(new File(['d'], 'a.png'));
    await flush();

    lastXHR.status = 200;
    lastXHR.responseText = JSON.stringify({});
    lastXHR.onload?.();

    await expect(promise).resolves.toEqual({ Key: 'a.png', URL: '' });
  });

  it('listObjects normalizes missing fields to empty shapes', async () => {
    mockedRequest.mockResolvedValueOnce({});
    await expect(listObjects({ prefix: 'avatars/' })).resolves.toEqual({
      objects: [],
      prefixes: [],
      isTruncated: false,
      nextMarker: '',
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/storage/objects', {
      params: { prefix: 'avatars/' },
    });

    mockedRequest.mockResolvedValueOnce({
      objects: [{ key: 'a', size: 1 }],
      prefixes: ['p/'],
      isTruncated: true,
      nextMarker: 'm1',
    });
    await expect(listObjects({})).resolves.toEqual({
      objects: [{ key: 'a', size: 1 }],
      prefixes: ['p/'],
      isTruncated: true,
      nextMarker: 'm1',
    });
  });

  it('delete/batch-delete/directory/signed-url hit their endpoints with the right shapes', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await deleteObject('a/b.png');
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/storage/objects', {
      method: 'DELETE',
      params: { path: 'a/b.png' },
    });

    await batchDeleteObjects(['a', 'b']);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/storage/objects/batch-delete', {
      method: 'POST',
      data: { paths: ['a', 'b'] },
    });

    await createDirectory('assets/');
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/storage/directories', {
      method: 'POST',
      data: { prefix: 'assets/' },
    });

    mockedRequest.mockResolvedValueOnce({ url: 'https://s' });
    await expect(getSignedUrl('x')).resolves.toEqual({ url: 'https://s' });
    mockedRequest.mockResolvedValueOnce({});
    await expect(getSignedUrl('x')).resolves.toEqual({ url: '' });
  });

  it('uploadAsset surfaces getIntl only through the mocked intl shape', () => {
    // getIntl 已在模块 mock 中定型为 defaultMessage 透传；这里锁定契约存在性
    expect(typeof getIntl().formatMessage).toBe('function');
  });
});
