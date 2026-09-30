/*
 * OpenAPISources shared.ts 纯函数专项测试
 *
 * errorMessage/diagnosticsFromError（含 isDiagnostic 过滤）、
 * diagnosticColor/diagnosticAlertType/riskColor/capabilityColor/
 * executionColor（缺省与各枚举臂）、formatDate（空/合法/非法）、
 * functionLabel（localizedText 三级回退）、operationLabel（三臂）、
 * proposalInboxPath（resourceKey 有无）、parseOpenAPIDocument
 * （合法与四类非法 JSON，错误文案经 getIntl()）。
 */
import { getIntl } from '@umijs/max';
import {
  capabilityColor,
  diagnosticAlertType,
  diagnosticColor,
  diagnosticsFromError,
  errorMessage,
  executionColor,
  formatDate,
  functionLabel,
  operationLabel,
  parseOpenAPIDocument,
  proposalInboxPath,
  riskColor,
} from '../shared';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { OpenAPISourceOperation } from '@/services/api/openapi';

jest.mock('@umijs/max', () => ({
  getIntl: () => ({
    formatMessage: (opts: { id: string; defaultMessage?: string }) =>
      opts.defaultMessage ?? opts.id,
  }),
}));

describe('errorMessage', () => {
  const fallback = '加载失败';

  it('Error.message 优先', () => {
    expect(errorMessage(new Error('boom'), fallback)).toBe('boom');
    expect(errorMessage(new Error(''), fallback)).toBe(fallback);
  });

  it('response.data.message 次之', () => {
    expect(errorMessage({ response: { data: { message: 'server msg' } } }, fallback)).toBe(
      'server msg',
    );
  });

  it('其余回退 fallback（null 与非 Error 对象）', () => {
    expect(errorMessage(null, fallback)).toBe(fallback);
    expect(errorMessage({ message: '' }, fallback)).toBe(fallback);
  });
});

describe('diagnosticsFromError', () => {
  it('details.diagnostics 数组过滤出合法项', () => {
    const out = diagnosticsFromError({
      response: {
        data: {
          details: {
            diagnostics: [
              { code: 'E1', severity: 'error', message: 'm1' },
              { code: 'E2', severity: 'error', message: 'm2', field: 'paths' },
              { notDiagnostic: true },
              null,
            ],
          },
        },
      },
    });
    expect(out).toHaveLength(2);
    expect(out[0].code).toBe('E1');
    expect(out[1].field).toBe('paths');
  });

  it('非数组/缺失 details → 空', () => {
    expect(diagnosticsFromError(new Error('x'))).toEqual([]);
    expect(
      diagnosticsFromError({ response: { data: { details: { diagnostics: 'nope' } } } }),
    ).toEqual([]);
    expect(diagnosticsFromError({ response: { data: {} } })).toEqual([]);
  });
});

describe('颜色与类型映射', () => {
  it('diagnosticColor：error/warning/其他（info/undefined）', () => {
    expect(diagnosticColor('error')).toBe('red');
    expect(diagnosticColor('warning')).toBe('orange');
    expect(diagnosticColor('info')).toBe('blue');
    expect(diagnosticColor()).toBe('blue');
  });

  it('diagnosticAlertType：error/warning/其他', () => {
    expect(diagnosticAlertType('error')).toBe('error');
    expect(diagnosticAlertType('warning')).toBe('warning');
    expect(diagnosticAlertType('info')).toBe('info');
    expect(diagnosticAlertType()).toBe('info');
  });

  it('riskColor：danger/high/warning/其他与缺省', () => {
    expect(riskColor('danger')).toBe('red');
    expect(riskColor('high')).toBe('volcano');
    expect(riskColor('warning')).toBe('orange');
    expect(riskColor('low')).toBe('green');
    expect(riskColor()).toBe('green');
  });

  it('capabilityColor：task/report/collection_query/item_query/CRUD/其他', () => {
    expect(capabilityColor('task')).toBe('purple');
    expect(capabilityColor('report')).toBe('geekblue');
    expect(capabilityColor('collection_query')).toBe('cyan');
    expect(capabilityColor('item_query')).toBe('cyan');
    expect(capabilityColor('create')).toBe('volcano');
    expect(capabilityColor('update')).toBe('volcano');
    expect(capabilityColor('delete')).toBe('volcano');
    expect(capabilityColor('list')).toBe('blue');
    expect(capabilityColor()).toBe('blue');
  });

  it('executionColor：task 紫/其他绿/缺省绿', () => {
    expect(executionColor('task')).toBe('purple');
    expect(executionColor('sync')).toBe('green');
    expect(executionColor()).toBe('green');
  });
});

describe('formatDate', () => {
  it('空 → "-"', () => {
    expect(formatDate()).toBe('-');
    expect(formatDate('')).toBe('-');
  });

  it('合法日期 → toLocaleString', () => {
    expect(formatDate('2026-09-30T08:00:00Z')).toBe(
      new Date('2026-09-30T08:00:00Z').toLocaleString(),
    );
  });

  it('非法日期 → 原值透传', () => {
    expect(formatDate('not-a-date')).toBe('not-a-date');
  });
});

describe('functionLabel（localizedText 三级回退）', () => {
  it('summary 命中 zh-CN → `${title} (${id})`', () => {
    const fn: FunctionDescriptor = {
      id: 'fn.a',
      summary: { 'zh-CN': '玩家', 'en-US': 'Player' },
      displayName: { 'zh-CN': '显示名', 'en-US': 'Display' },
    };
    expect(functionLabel(fn)).toBe('玩家 (fn.a)');
  });

  it('summary 空串 → displayName 回退', () => {
    expect(
      functionLabel({
        id: 'fn.b',
        summary: { 'zh-CN': '', 'en-US': '' },
        displayName: { 'zh-CN': '显示名', 'en-US': '' },
      }),
    ).toBe('显示名 (fn.b)');
  });

  it('两者皆空 → id 兜底；zh-CN 缺失回退 en-US', () => {
    expect(functionLabel({ id: 'fn.c' })).toBe('fn.c (fn.c)');
    // summary 无 zh-CN 时 localizedText 回退 en-US（非 id 兜底臂）。
    expect(functionLabel({ id: 'fn.d', summary: { 'en-US': 'OnlyEn' } })).toBe('OnlyEn (fn.d)');
    // summary 整体缺失 + displayName 空串 → id 兜底。
    expect(
      functionLabel({
        id: 'fn.e',
        displayName: { 'zh-CN': '', 'en-US': '' },
      }),
    ).toBe('fn.e (fn.e)');
  });
});

describe('operationLabel（三臂）', () => {
  const op = (over: Partial<OpenAPISourceOperation>): OpenAPISourceOperation => ({
    operationId: 'op-x',
    method: 'GET',
    path: '/x',
    approval: { required: false } as OpenAPISourceOperation['approval'],
    bound: false,
    ...over,
  });

  it('summary 优先，其次 operation，最后 operationId', () => {
    expect(operationLabel(op({ summary: 'S', operation: 'list' }))).toBe('S');
    expect(operationLabel(op({ operation: 'list' }))).toBe('list');
    expect(operationLabel(op({}))).toBe('op-x');
    expect(operationLabel(op({ summary: '', operation: '' }))).toBe('op-x');
  });
});

describe('proposalInboxPath', () => {
  it('带 resourceKey：resourceKey+proposalKey 顺序拼接', () => {
    expect(proposalInboxPath('pk-1', 'rk-9')).toBe(
      '/functions/pages?resourceKey=rk-9&proposalKey=pk-1',
    );
  });

  it('缺 resourceKey：只带 proposalKey', () => {
    expect(proposalInboxPath('pk-1')).toBe('/functions/pages?proposalKey=pk-1');
  });
});

describe('parseOpenAPIDocument', () => {
  const valid = JSON.stringify({ openapi: '3.0.0', info: { title: 'x', version: '1' } });

  it('合法对象原样返回', () => {
    expect(parseOpenAPIDocument(valid)).toEqual(JSON.parse(valid));
  });

  it('非法 JSON → SyntaxError 透传（不经 getIntl）', () => {
    expect(() => parseOpenAPIDocument('{oops')).toThrow(SyntaxError);
  });

  it('非对象（数组/裸值）→ 必须是对象', () => {
    expect(() => parseOpenAPIDocument('[1,2]')).toThrow('OpenAPI JSON 必须是对象');
    expect(() => parseOpenAPIDocument('"str"')).toThrow('OpenAPI JSON 必须是对象');
    expect(() => parseOpenAPIDocument('null')).toThrow('OpenAPI JSON 必须是对象');
  });

  it('缺 openapi 字段', () => {
    expect(() => parseOpenAPIDocument(JSON.stringify({ info: {} }))).toThrow(
      'OpenAPI JSON 缺少 openapi 字段',
    );
    expect(() => parseOpenAPIDocument(JSON.stringify({ openapi: '  ', info: {} }))).toThrow(
      'OpenAPI JSON 缺少 openapi 字段',
    );
  });

  it('缺 info 对象（含 info 为数组）', () => {
    expect(() => parseOpenAPIDocument(JSON.stringify({ openapi: '3.0.0' }))).toThrow(
      'OpenAPI JSON 缺少 info 对象',
    );
    expect(() => parseOpenAPIDocument(JSON.stringify({ openapi: '3.0.0', info: [] }))).toThrow(
      'OpenAPI JSON 缺少 info 对象',
    );
  });

  it('getIntl 词条存在性（错误文案经模块级 getIntl 解析）', () => {
    const intl = getIntl();
    expect(intl.formatMessage({ id: 'x' })).toBeTruthy();
  });
});
