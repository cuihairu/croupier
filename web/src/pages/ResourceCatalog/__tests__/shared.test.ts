/** 资源目录 shared.ts 纯函数：语义展示/冲突来源/表单转换/提交载荷压缩。 */
import {
  bindingFreshnessSummary,
  compactSemanticsPayload,
  conflictSources,
  displaySemanticValue,
  pageTitleText,
  semanticsToFormValues,
} from '../shared';
import type {
  AffectedPageInfo,
  SemanticsInfo,
  SemanticConflictInfo,
  UpdateResourceSemanticsRequest,
} from '@/types/dashboard';

const intl = {
  formatMessage: ({ defaultMessage }: { id: string; defaultMessage: string }) => defaultMessage,
};

describe('displaySemanticValue', () => {
  it('空值返回 -', () => {
    expect(displaySemanticValue(undefined)).toBe('-');
    expect(displaySemanticValue('')).toBe('-');
  });

  it('JSON 字符串解包为原文、对象再序列化、非 JSON 原样返回', () => {
    expect(displaySemanticValue('"elite"')).toBe('elite');
    expect(displaySemanticValue('{"a":1}')).toBe('{"a":1}');
    expect(displaySemanticValue('raw-value')).toBe('raw-value');
  });
});

describe('pageTitleText', () => {
  it('优先取本地化标题，缺标题回退 -', () => {
    const page: AffectedPageInfo = {
      pageKey: 'resource--players',
      kind: 'resource' as never,
      title: { 'zh-CN': '玩家目录' },
    };
    expect(pageTitleText(page)).toBe('玩家目录');
    expect(pageTitleText(undefined)).toBe('-');
  });
});

describe('bindingFreshnessSummary', () => {
  it('无诊断返回「无」，有诊断拼接状态串', () => {
    expect(bindingFreshnessSummary(intl, undefined)).toBe('无');
    const page: AffectedPageInfo = {
      pageKey: 'p',
      kind: 'resource' as never,
      bindingFreshness: [{ status: 'fresh' }, { status: 'stale' }],
    } as never;
    expect(bindingFreshnessSummary(intl, page)).toBe('fresh, stale');
  });
});

describe('conflictSources', () => {
  it('按 values 中存在的来源过滤', () => {
    const conflict: SemanticConflictInfo = {
      field: 'items',
      values: { platform_review: 'a', openapi_rest: 'b' },
    };
    expect(conflictSources(conflict)).toEqual(['platform_review', 'openapi_rest']);
  });
});

describe('semanticsToFormValues', () => {
  it('缺省输入填默认值（类型 string、集合空数组）', () => {
    expect(semanticsToFormValues(undefined)).toEqual({
      identityField: undefined,
      identityFieldType: 'string',
      identityPath: undefined,
      collectionQueryId: undefined,
      collectionPath: undefined,
      pageFieldName: undefined,
      pageSizeFieldName: undefined,
      itemsFieldName: undefined,
      totalFieldName: undefined,
      itemQueryId: undefined,
      itemPath: undefined,
      createId: undefined,
      updateId: undefined,
      deleteId: undefined,
      actions: [],
      tasks: [],
      reports: [],
    });
  });

  it('已有语义透传各字段', () => {
    const semantics = {
      identityField: 'player_id',
      identityFieldType: 'number',
      identityPath: '/id',
      collectionQueryId: 7,
      itemsFieldName: 'rows',
      createId: 1,
      actions: [],
      tasks: [],
      reports: [],
    } as unknown as SemanticsInfo;
    const values = semanticsToFormValues(semantics);
    expect(values.identityFieldType).toBe('number');
    expect(values.collectionQueryId).toBe(7);
    expect(values.itemsFieldName).toBe('rows');
    expect(values.createId).toBe(1);
  });
});

describe('compactSemanticsPayload', () => {
  it('裁剪空白、丢弃空串与非正数、压缩 actions/tasks/reports', () => {
    const values: UpdateResourceSemanticsRequest = {
      identityField: '  player_id  ',
      identityFieldType: 'string',
      identityPath: '   ',
      collectionQueryId: 3,
      itemQueryId: 0,
      createId: 1,
      deleteId: -1,
      changeReason: ' 调整语义 ',
      actions: [
        { functionId: ' a.fn ', subject: 'resource_item', identityInput: ' id ' },
        { functionId: '   ', subject: 'none' },
      ],
      tasks: [
        {
          start: { functionId: ' t.start ' },
          taskId: { resultPath: '/id', valueType: 'string' },
          status: {
            function: { functionId: ' t.status ' },
            taskIdInput: '/in',
            statePath: '/state',
          },
          events: {
            function: { functionId: ' t.events ' },
            taskIdInput: ' /ein ',
            eventsPath: ' /ev ',
          },
          result: {
            function: { functionId: 't.result' },
            taskIdInput: '/rin',
            resultPath: ' /res ',
          },
          cancel: { function: { functionId: 't.cancel' }, taskIdInput: ' /cin ' },
        },
        {
          start: { functionId: '  ' },
          taskId: { resultPath: '/id', valueType: 'string' },
          status: { function: { functionId: 't.status' }, taskIdInput: '/in', statePath: '/state' },
        },
      ],
      reports: [
        {
          query: { functionId: ' r.fn ' },
          datasetPath: ' /ds ',
          dimensions: [' /d1 ', '  ', ' /d2 '],
          metrics: [' /m1 '],
        },
        { query: { functionId: '  ' }, datasetPath: '/x', dimensions: [], metrics: [] },
      ],
    };

    const payload = compactSemanticsPayload(values);

    expect(payload).toEqual({
      identityField: 'player_id',
      identityFieldType: 'string',
      collectionQueryId: 3,
      createId: 1,
      changeReason: '调整语义',
      actions: [{ functionId: 'a.fn', subject: 'resource_item', identityInput: 'id' }],
      tasks: [
        {
          start: { functionId: 't.start' },
          taskId: { resultPath: '/id', valueType: 'string' },
          status: { function: { functionId: 't.status' }, taskIdInput: '/in', statePath: '/state' },
          events: { function: { functionId: 't.events' }, taskIdInput: '/ein', eventsPath: '/ev' },
          result: { function: { functionId: 't.result' }, taskIdInput: '/rin', resultPath: '/res' },
          cancel: { function: { functionId: 't.cancel' }, taskIdInput: '/cin' },
        },
      ],
      reports: [
        {
          query: { functionId: 'r.fn' },
          datasetPath: '/ds',
          dimensions: ['/d1', '/d2'],
          metrics: ['/m1'],
        },
      ],
    });
    // 空串/非正数不落入载荷
    expect(payload.identityPath).toBeUndefined();
    expect(payload.itemQueryId).toBeUndefined();
    expect(payload.deleteId).toBeUndefined();
  });

  it('任务可选子字段全缺省：taskId/status/events/result/cancel 逐项走 || 兜底', () => {
    const payload = compactSemanticsPayload({
      tasks: [
        {
          start: { functionId: '  t.optional  ' },
          status: { function: { functionId: ' t.opt.status ' } },
          events: { function: { functionId: ' t.ev ' } },
          result: { function: { functionId: ' t.res ' } },
          cancel: { function: { functionId: ' t.cancel ' } },
        },
      ],
    } as unknown as UpdateResourceSemanticsRequest);

    expect(payload.tasks).toEqual([
      {
        start: { functionId: 't.optional' },
        taskId: { resultPath: '', valueType: 'string' },
        status: { function: { functionId: 't.opt.status' }, taskIdInput: '', statePath: '' },
        events: { function: { functionId: 't.ev' }, taskIdInput: '', eventsPath: '' },
        result: { function: { functionId: 't.res' }, taskIdInput: '', resultPath: '' },
        cancel: { function: { functionId: 't.cancel' }, taskIdInput: '' },
      },
    ]);
  });

  it('报表 query/datasetPath/dimensions/metrics 缺省：空 functionId 报表被丢弃', () => {
    const payload = compactSemanticsPayload({
      reports: [
        { query: { functionId: '   ' } },
        {
          query: { functionId: ' r.two ' },
          datasetPath: ' /ds2 ',
          dimensions: [' d1 ', '  ', ' d2 '],
          metrics: [' m1 '],
        },
        { query: {} },
      ],
    } as unknown as UpdateResourceSemanticsRequest);

    expect(payload.reports).toEqual([
      {
        query: { functionId: 'r.two' },
        datasetPath: '/ds2',
        dimensions: ['d1', 'd2'],
        metrics: ['m1'],
      },
    ]);
  });
});

describe('pageTitleText / bindingFreshnessSummary 兜底', () => {
  it('标题缺失时 localizedText 兜底恒为占位串（pageKey 分支见不可达分支说明）', () => {
    expect(pageTitleText({ pageKey: 'resource--players', kind: 'draft' } as AffectedPageInfo)).toBe(
      '-',
    );
    expect(pageTitleText({} as AffectedPageInfo)).toBe('-');
  });

  it('bindingFreshness 存在但为空数组同样返回「无」', () => {
    expect(
      bindingFreshnessSummary(intl, {
        pageKey: 'p',
        kind: 'draft',
        bindingFreshness: [],
      } as AffectedPageInfo),
    ).toBe('无');
  });

  it('bindingFreshness 只有单项时直接返回该状态', () => {
    expect(
      bindingFreshnessSummary(intl, {
        pageKey: 'p',
        kind: 'draft',
        bindingFreshness: [{ status: 'stale' }],
      } as unknown as AffectedPageInfo),
    ).toBe('stale');
  });
});
