import { request } from '@umijs/max';
import {
  bindOpenAPISourceProvider,
  createOpenAPISource,
  deleteOpenAPISourceBinding,
  descriptorToOpenAPI,
  extractOpenAPIMetadata,
  getFunctionOpenAPI,
  getOpenAPISource,
  getOpenAPISourceDiagnostics,
  listOpenAPISources,
  listRuntimeSources,
  normalizeFunctionOpenAPIResponse,
  updateOpenAPISource,
  uploadOpenAPISourceFile,
  type GetFunctionOpenAPIResponse,
  type OpenAPIDocument,
  type OpenAPIOperation,
  type RuntimeSourcesListResponse,
} from './openapi';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

const spec: OpenAPIDocument = {
  openapi: '3.0.3',
  info: { title: 'Demo', version: '1.0.0' },
  paths: { '/players/{id}/ban': { post: { operationId: 'player.ban' } } },
};

describe('normalizeFunctionOpenAPIResponse', () => {
  it('unwraps the { spec } envelope from the backend response', () => {
    const operation: OpenAPIOperation = { operationId: 'player.ban', summary: 'ban' };
    expect(normalizeFunctionOpenAPIResponse({ spec: operation })).toBe(operation);
  });

  it('returns a bare operation object unchanged', () => {
    const operation: OpenAPIOperation = { operationId: 'noop' };
    expect(normalizeFunctionOpenAPIResponse(operation)).toBe(operation);
  });

  it('returns an empty object when spec is present but falsy', () => {
    expect(
      normalizeFunctionOpenAPIResponse({
        spec: undefined,
      } as unknown as GetFunctionOpenAPIResponse),
    ).toEqual({});
  });

  it('returns an empty object for falsy responses', () => {
    expect(normalizeFunctionOpenAPIResponse(undefined)).toEqual({});
    expect(normalizeFunctionOpenAPIResponse(null as unknown as GetFunctionOpenAPIResponse)).toEqual(
      {},
    );
  });

  it('returns non-object responses as-is (defensive passthrough)', () => {
    expect(normalizeFunctionOpenAPIResponse('raw' as unknown as OpenAPIOperation)).toBe('raw');
  });
});

describe('openapi API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('getFunctionOpenAPI unwraps the spec envelope', async () => {
    const operation: OpenAPIOperation = { operationId: 'player.ban' };
    mockedRequest.mockResolvedValue({ spec: operation });

    await expect(getFunctionOpenAPI('player.ban')).resolves.toBe(operation);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player.ban/openapi');
  });

  it('createOpenAPISource posts the document with an optional name', async () => {
    const source = { source: { sourceId: 'src-1', operations: [] } };
    mockedRequest.mockResolvedValue(source);

    await expect(createOpenAPISource(spec, 'demo-spec')).resolves.toBe(source);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/openapi/sources', {
      method: 'POST',
      data: { name: 'demo-spec', spec },
    });
  });

  it('uploadOpenAPISourceFile appends the file and optional name as form data', async () => {
    const file = new File(['openapi: 3.0.3'], 'spec.yaml', { type: 'text/yaml' });
    mockedRequest.mockResolvedValue({});

    await uploadOpenAPISourceFile(file, 'imported');

    const options = mockedRequest.mock.calls[0][1] as {
      method: string;
      data: FormData;
      requestType: string;
    };
    expect(mockedRequest.mock.calls[0][0]).toBe('/api/v1/openapi/sources');
    expect(options.method).toBe('POST');
    expect(options.requestType).toBe('form');
    expect(options.data.get('file')).toBe(file);
    expect(options.data.get('name')).toBe('imported');
  });

  it('uploadOpenAPISourceFile omits the name field when not provided', async () => {
    const file = new File(['openapi: 3.0.3'], 'spec.yaml');
    mockedRequest.mockResolvedValue({});

    await uploadOpenAPISourceFile(file);

    const options = mockedRequest.mock.calls[0][1] as { data: FormData };
    expect(options.data.get('file')).toBe(file);
    expect(options.data.get('name')).toBeNull();
  });

  it('updateOpenAPISource PUTs the document with URL-encoded id', async () => {
    mockedRequest.mockResolvedValue({});

    await updateOpenAPISource('src/1', spec, 'renamed');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/openapi/sources/src%2F1', {
      method: 'PUT',
      data: { name: 'renamed', spec },
    });
  });

  it('listOpenAPISources GETs the source list', async () => {
    const list = { items: [] };
    mockedRequest.mockResolvedValue(list);

    await expect(listOpenAPISources()).resolves.toBe(list);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/openapi/sources');
  });

  it('getOpenAPISource URL-encodes the source id', async () => {
    const detail = { source: { sourceId: 'src/1', operations: [] } };
    mockedRequest.mockResolvedValue(detail);

    await expect(getOpenAPISource('src/1')).resolves.toBe(detail);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/openapi/sources/src%2F1');
  });

  it('getOpenAPISourceDiagnostics fetches diagnostics for the source', async () => {
    const diagnostics = { sourceId: 'src-1', diagnostics: [] };
    mockedRequest.mockResolvedValue(diagnostics);

    await expect(getOpenAPISourceDiagnostics('src-1')).resolves.toBe(diagnostics);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/openapi/sources/src-1/diagnostics');
  });

  it('bindOpenAPISourceProvider posts a provider binding merged with kind', async () => {
    const binding = { binding: { bindingId: 'b-1', operationId: 'player.ban', kind: 'provider' } };
    mockedRequest.mockResolvedValue(binding);

    await expect(
      bindOpenAPISourceProvider('src-1', {
        operationId: 'player.ban',
        functionId: 'fn-1',
        providerId: 'prov-1',
      }),
    ).resolves.toBe(binding);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/openapi/sources/src-1/bindings', {
      method: 'POST',
      data: {
        operationId: 'player.ban',
        functionId: 'fn-1',
        providerId: 'prov-1',
        kind: 'provider',
      },
    });
  });

  it('deleteOpenAPISourceBinding URL-encodes both ids', async () => {
    mockedRequest.mockResolvedValue({});

    await deleteOpenAPISourceBinding('src/1', 'b/2');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/openapi/sources/src%2F1/bindings/b%2F2', {
      method: 'DELETE',
    });
  });

  it('listRuntimeSources GETs agent runtime providers', async () => {
    const list: RuntimeSourcesListResponse = {
      items: [
        {
          providerId: 'prov-1',
          name: 'demo-provider',
          agentId: 'agent-1',
          gameId: 'demo',
          env: 'prod',
          version: 'v1',
          functionCount: 2,
          functions: ['player.list', 'player.ban'],
          lastSeenUnix: 1758123456,
        },
      ],
      total: 1,
    };
    mockedRequest.mockResolvedValue(list);

    await expect(listRuntimeSources()).resolves.toBe(list);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/openapi/runtime-sources');
  });
});

describe('descriptorToOpenAPI', () => {
  it('projects a fully-populated descriptor with all extension fields', () => {
    expect(
      descriptorToOpenAPI({
        id: 'player.ban',
        summary: { 'zh-CN': '封禁玩家', 'en-US': 'Ban Player' },
        description: '封禁指定玩家',
        tags: ['gm', 'player'],
        resource: 'player',
        capability: 'action',
        execution: 'sync',
        risk: 'high',
        operation: 'ban',
        enabled: false,
        permission: 'player:ban',
      }),
    ).toEqual({
      operationId: 'player.ban',
      summary: '封禁玩家',
      description: '封禁指定玩家',
      tags: ['gm', 'player'],
      extensions: {
        'x-resource': 'player',
        'x-capability': 'action',
        'x-execution': 'sync',
        'x-risk': 'high',
        'x-operation': 'ban',
        'x-enabled': false,
        'x-permission': 'player:ban',
      },
    });
  });

  it('falls back summary to the id and description to the summary for a minimal descriptor', () => {
    expect(descriptorToOpenAPI({ id: 'noop' })).toEqual({
      operationId: 'noop',
      summary: 'noop',
      description: '',
      tags: undefined,
    });
  });

  it('derives description from the localized summary when description is absent', () => {
    const op = descriptorToOpenAPI({
      id: 'player.kick',
      summary: { 'zh-CN': '踢出玩家', 'en-US': 'Kick' },
    });
    expect(op.summary).toBe('踢出玩家');
    expect(op.description).toBe('踢出玩家');
  });

  it('derives tags from the resource when tags are absent', () => {
    expect(descriptorToOpenAPI({ id: 'mail.send', resource: 'mail' }).tags).toEqual(['mail']);
  });
});

describe('extractOpenAPIMetadata', () => {
  it('reads every extension field back out of the operation', () => {
    expect(
      extractOpenAPIMetadata({
        extensions: {
          'x-resource': 'player',
          'x-capability': 'action',
          'x-execution': 'task',
          'x-risk': 'danger',
          'x-operation': 'ban',
          'x-enabled': true,
          'x-permission': 'player:ban',
        },
      }),
    ).toEqual({
      resource: 'player',
      capability: 'action',
      execution: 'task',
      risk: 'danger',
      operation: 'ban',
      enabled: true,
      permission: 'player:ban',
    });
  });

  it('returns undefined metadata when the operation has no extensions', () => {
    expect(extractOpenAPIMetadata({ operationId: 'noop' })).toEqual({
      resource: undefined,
      capability: undefined,
      execution: undefined,
      risk: undefined,
      operation: undefined,
      enabled: undefined,
      permission: undefined,
    });
  });
});
