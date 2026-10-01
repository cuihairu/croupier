/**
 * 函数详情页数据层 hook 单测（覆盖率补缺：useFunctionDetailPage.ts 498 行
 * 0% → 收口——v8 定向扫描确证 1-498 全零；域内既有 5 个测试全打
 * DetailSections/DetailTabs 子组件，无人 import 本 hook，import 面核验
 * 后认领）。
 *
 * 锁定契约：
 * - parsedInputSchema 七级回退链（detail.inputSchema → index.inputSchema
 *   → detail.schema → index.schema → detail.params → index.params →
 *   openapi bodySchema）与 parseMaybeJSON 四翼（字符串合法对象 / 字符串
 *   数组拒斥 / 坏 JSON 捕获 undefined / 非字符串对象直返 / falsy 短路）；
 * - effectiveResource 回退（direct → x-resource → ''）；
 * - toDescriptorArray 三翼（裸数组 / functions 信封 / descriptors 信封 /
 *   空对象兜底 []）；
 * - loadSourceOfTruth allSettled 四翼（双 fulfilled / descs rejected →
 *   null / openapi rejected → null）；
 * - loadDetail 成功归一链（displayName→id 本地化回退、summary→description
 *   描述链、resource||indexItem 回退、tags join 进 form、perm items
 *   非空直用 / 空时默认 function·invoke 行、permError 清空）；
 * - loadDetail 失败链（perm Error.message / 非 Error intl 兜底；detail
 *   400/404 降级：desc 命中 → 运行时形态 + runtimeNotSupported、
 *   desc 未命中 → 函数不存在、listDescriptors 再炸 → 加载函数详情失败；
 *   非 400/404 → 加载函数详情失败；functionId 缺省早退）；
 * - handlers 五链（save tags split/trim/filter + 成功/失败、statusToggle
 *   enable/disable 双翼 + 失败、copy 成功跳新 ID/失败、delete confirm
 *   onOk 成功跳目录/失败、savePermissions 成功/Error message/非 Error
 *   兜底）与 functionId 卫兵；
 * - contractDiagnostics 透传（indexDesc.diagnostics ?? []）。
 *
 * mock 口径：services 十函数全 jest.mock；antd 经 requireActual 保留组件
 * 真实，覆写 App.useApp（message/modal 走模块级共享桩 mockAppStubs，
 * beforeEach clearAllMocks 清零，断言直接 toHaveBeenCalledWith——规避
 * jsdom 下 antd message/modal DOM 残留跨用例串扰，坑 8b 同源；modal
 * .confirm 不渲染 DOM，onOk 回调取自 confirm.mock.calls[0][0] 手动驱动）
 * 与 Form.useForm（useRef 保持的内存 store——真实 useForm 实例未连接
 * <Form form={...}> 时 getFieldsValue()/validateFields() 返回 {}，
 * hook 单测无表单挂载面，setFieldsValue 合并/直返 store 等价替代）。
 * @umijs/max 工厂自含 makeIntl（values 插值）+ history.push 桩。
 *
 * 边界（诚实，登记结构不可达翼——三组，v8 分支 97.4% 的全部缺口）：
 * 1. loadSourceOfTruth 外层 try/catch（L188-191）——Promise.allSettled
 *    永不 reject，catch 体不可达（防御式）；
 * 2. effectiveResource 的 fromIndex / fromDetailDesc 两级（L128/L130）——
 *    loadDetail 归一时 `resource: detail.resource || indexItem?.resource`
 *    已把 index 值并入 functionDetail.resource，direct 级恒先命中
 *    （descriptor 即 detail 原对象，fromDetailDesc 同理被 direct 遮蔽），
 *    页面链不可达；
 * 3. jsonViewData.function.tags 的 `functionDetail.tags || []` 右翼
 *    （L147）——归一链恒产数组（空数组亦 truthy），undefined 形态不可达。
 */
import { renderHook, act, waitFor } from '@testing-library/react';
import useFunctionDetailPage from '../useFunctionDetailPage';
import {
  getFunctionDetail,
  getFunctionOpenAPI,
  updateFunction,
  deleteFunction,
  copyFunction,
  enableFunction,
  disableFunction,
  getFunctionPermissions,
  updateFunctionPermissions,
  listDescriptors,
} from '@/services/api/functions';
import type { FunctionDescriptor, FunctionPermission } from '@/services/api/functions';
import { history } from '@umijs/max';

jest.setTimeout(30000);

// 模块级共享桩：mock 工厂每次 useApp() 返回同一引用，断言可直达
// （mock* 前缀变量：babel-jest hoist 白名单）
const mockAppStubs = {
  message: { success: jest.fn(), error: jest.fn() },
  modal: { confirm: jest.fn() },
};

// antd：组件真实，覆写 App.useApp（message/modal 直桩断言）与
// Form.useForm（受控 store）——antd 真实 useForm 实例在未连接
// <Form form={...}> 时 getFieldsValue()/validateFields() 返回 {}，
// hook 单测无表单挂载面，用 useRef 保持的内存 store 等价替代
// （setFieldsValue 合并 / getFieldsValue / validateFields 直返 store）
jest.mock('antd', () => {
  const actual = jest.requireActual('antd');
  const R = require('react');
  const useForm = () => {
    const storeRef = R.useRef<Record<string, unknown> | null>(null);
    if (!storeRef.current) {
      let store: Record<string, unknown> = {};
      storeRef.current = {
        setFieldsValue: (v: Record<string, unknown>) => {
          store = { ...store, ...v };
        },
        getFieldsValue: () => store,
        validateFields: async () => store,
      };
    }
    return [storeRef.current, R.useRef(null)] as const;
  };
  return {
    ...actual,
    App: {
      ...actual.App,
      useApp: () => mockAppStubs,
    },
    Form: {
      ...actual.Form,
      useForm,
    },
  };
});

jest.mock('@umijs/max', () => {
  const makeIntl = () => ({
    locale: 'zh-CN',
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return text;
    },
  });
  return {
    useIntl: makeIntl,
    getIntl: makeIntl,
    history: { push: jest.fn() },
  };
});

jest.mock('@/services/api/functions', () => ({
  __esModule: true,
  getFunctionDetail: jest.fn(),
  getFunctionOpenAPI: jest.fn(),
  updateFunction: jest.fn(),
  deleteFunction: jest.fn(),
  copyFunction: jest.fn(),
  enableFunction: jest.fn(),
  disableFunction: jest.fn(),
  getFunctionPermissions: jest.fn(),
  updateFunctionPermissions: jest.fn(),
  listDescriptors: jest.fn(),
}));

const mDetail = jest.mocked(getFunctionDetail);
const mOpenapi = jest.mocked(getFunctionOpenAPI);
const mUpdate = jest.mocked(updateFunction);
const mDelete = jest.mocked(deleteFunction);
const mCopy = jest.mocked(copyFunction);
const mEnable = jest.mocked(enableFunction);
const mDisable = jest.mocked(disableFunction);
const mGetPerm = jest.mocked(getFunctionPermissions);
const mUpdatePerm = jest.mocked(updateFunctionPermissions);
const mDescs = jest.mocked(listDescriptors);
const mPush = history.push as jest.Mock;
const mSuccess = mockAppStubs.message.success;
const mError = mockAppStubs.message.error;
const mConfirm = mockAppStubs.modal.confirm;

// 成功链默认 detail：displayName/summary 本地化齐备
const baseDetail = {
  id: 'fn.main',
  displayName: { 'zh-CN': '主函数', 'en-US': 'Main' },
  summary: { 'zh-CN': '主函数摘要' },
  description: { 'zh-CN': '主函数描述' },
  resource: 'player',
  operation: 'query',
  version: '2.1.0',
  tags: ['hot', 'read'],
};

const baseIndexDesc: FunctionDescriptor = { id: 'fn.main', version: '2.1.0' };

const basePerms = {
  items: [{ resource: 'function', actions: ['invoke'], roles: ['ops'] }] as FunctionPermission[],
};

function renderDetailHook(functionId?: string) {
  return renderHook(() => useFunctionDetailPage(functionId));
}

type HookResult = ReturnType<typeof useFunctionDetailPage>;

/** 等待主加载链 settle（functionDetail 就位） */
async function settleMain(result: { current: HookResult }) {
  await waitFor(() => expect(result.current.functionDetail).not.toBeNull());
}

beforeEach(() => {
  jest.clearAllMocks();
  mDetail.mockResolvedValue(baseDetail as never);
  mOpenapi.mockResolvedValue({ extensions: {}, requestBody: undefined } as never);
  mDescs.mockResolvedValue([baseIndexDesc] as never);
  mGetPerm.mockResolvedValue(basePerms as never);
});

describe('解析与回退链', () => {
  it('parsedInputSchema：坏 JSON 拒斥后 detail.schema 命中（parse 三翼）', async () => {
    mDetail.mockResolvedValue({
      ...baseDetail,
      inputSchema: '{bad json', // 坏 JSON → catch → undefined
      schema: '{"type":"object","title":"S1"}', // 字符串合法对象 → 命中
    } as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.parsedInputSchema).toEqual({ type: 'object', title: 'S1' });
  });

  it('parsedInputSchema：非字符串数组直入拒斥翼（parseMaybeJSON L71-72）', async () => {
    mDetail.mockResolvedValue({ ...baseDetail, inputSchema: [1, 2] } as never);
    mDescs.mockResolvedValue([
      { ...baseIndexDesc, inputSchema: { type: 'object', title: 'S3' } },
    ] as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.parsedInputSchema).toEqual({ type: 'object', title: 'S3' });
  });

  it('parsedInputSchema：字符串数组拒斥翼 + index.inputSchema 对象直返命中', async () => {
    mDetail.mockResolvedValue({ ...baseDetail, inputSchema: '[1,2]' } as never);
    mDescs.mockResolvedValue([
      { ...baseIndexDesc, inputSchema: { type: 'object', title: 'S2' } },
    ] as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.parsedInputSchema).toEqual({ type: 'object', title: 'S2' });
  });

  it('parsedInputSchema：前四级空 → index.params 命中；effectiveResource direct 命中', async () => {
    mDetail.mockResolvedValue({ ...baseDetail, inputSchema: '', schema: null } as never);
    mDescs.mockResolvedValue([
      { ...baseIndexDesc, params: { type: 'array', title: 'P' } },
    ] as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.parsedInputSchema).toEqual({ type: 'array', title: 'P' });
    expect(result.current.effectiveResource).toBe('player');
  });

  it('parsedInputSchema：六源皆空 → openapi bodySchema 命中 + x-resource 回退', async () => {
    mDetail.mockResolvedValue({ ...baseDetail, resource: '' } as never);
    mDescs.mockResolvedValue([{ id: 'fn.main' }] as never);
    mOpenapi.mockResolvedValue({
      extensions: { 'x-resource': 'mail' },
      requestBody: { content: { 'application/json': { schema: { type: 'object', title: 'B' } } } },
    } as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.parsedInputSchema).toEqual({ type: 'object', title: 'B' });
    // resource 三前级全空（detail 空 + index 无）→ x-resource 命中
    expect(result.current.effectiveResource).toBe('mail');
  });

  it('parsedInputSchema 七源全空 → undefined；effectiveResource "" 兜底', async () => {
    mDetail.mockResolvedValue({ ...baseDetail, resource: '' } as never);
    mDescs.mockResolvedValue([{ id: 'fn.main' }] as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.parsedInputSchema).toBeUndefined();
    expect(result.current.effectiveResource).toBe('');
  });

  it('toDescriptorArray：functions / descriptors 信封 + 空对象兜底 []（经 jsonViewData 断言）', async () => {
    mDescs.mockResolvedValue({ functions: [{ ...baseIndexDesc, version: '9.9.9' }] } as never);
    const r1 = renderDetailHook('fn.main');
    await settleMain(r1.result);
    expect(r1.result.current.jsonViewData.descriptorFromIndexApi).toMatchObject({
      id: 'fn.main',
      version: '9.9.9',
    });
    r1.unmount();

    mDescs.mockResolvedValue({ descriptors: [baseIndexDesc] } as never);
    const r2 = renderDetailHook('fn.main');
    await settleMain(r2.result);
    expect(r2.result.current.jsonViewData.descriptorFromIndexApi).toMatchObject({ id: 'fn.main' });
    r2.unmount();

    mDescs.mockResolvedValue({} as never);
    const r3 = renderDetailHook('fn.main');
    await settleMain(r3.result);
    expect(r3.result.current.jsonViewData.descriptorFromIndexApi).toBeNull();
  });

  it('loadSourceOfTruth：openapi fulfilled(null) → openapiOperation null 翼', async () => {
    mOpenapi.mockResolvedValue(null as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.jsonViewData.openapiOperation).toBeNull();
  });

  it('loadSourceOfTruth rejected 双翼：descs 炸 → index null；openapi 炸 → operation null', async () => {
    mDescs.mockRejectedValue(new Error('descs down'));
    mOpenapi.mockRejectedValue(new Error('openapi down'));
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.jsonViewData.descriptorFromIndexApi).toBeNull();
    expect(result.current.jsonViewData.openapiOperation).toBeNull();
  });

  it('jsonViewData 组装 + formDescriptor 合并（index 覆盖 detail，resource/operation 回退）', async () => {
    mDetail.mockResolvedValue({ ...baseDetail, operation: undefined } as never);
    mDescs.mockResolvedValue([
      { ...baseIndexDesc, resource: 'idx-res', operation: 'idx-op' },
    ] as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);

    const jv = result.current.jsonViewData;
    expect(jv.function).toMatchObject({
      id: 'fn.main',
      name: '主函数',
      resource: 'player', // detail.resource 优先
      enabled: true,
      tags: ['hot', 'read'],
    });
    expect(jv.descriptorFromDetailApi).toMatchObject({ id: 'fn.main' });
    expect(jv.openapiOperation).toEqual({ extensions: {}, requestBody: undefined });

    const fd = result.current.formDescriptor;
    expect(fd.resource).toBe('idx-res'); // index.resource || detail.resource（index 覆盖）
    expect(fd.operation).toBe('idx-op'); // index.operation || detail.operation
    expect(fd.id).toBe('fn.main');
  });
});

describe('loadDetail 主链', () => {
  it('成功归一：本地化回退链 + resource||index 回退 + tags 进 form + perm 非空直用', async () => {
    mDetail.mockResolvedValue({ ...baseDetail, resource: '', displayName: undefined } as never);
    mDescs.mockResolvedValue([{ ...baseIndexDesc, resource: 'from-index' }] as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);

    const fd = result.current.functionDetail;
    expect(fd?.name).toBe('fn.main'); // displayName 缺 → id 回退
    expect(fd?.description).toBe('主函数摘要'); // summary 优先于 description
    expect(fd?.resource).toBe('from-index'); // detail 空 → index 回退翼
    expect(fd?.version).toBe('2.1.0');
    expect(fd?.descriptor).toMatchObject({ id: 'fn.main' });
    expect(fd?.health).toBe('healthy');

    expect(result.current.form.getFieldsValue()).toEqual({
      name: 'fn.main',
      description: '主函数摘要',
      resource: 'from-index',
      tags: 'hot, read',
    });
    // perm 非空直用：items 原样入表单
    expect(result.current.permForm.getFieldsValue().items).toEqual(basePerms.items);
    expect(result.current.permError).toBe('');
    expect(result.current.loading).toBe(false);
  });

  it('description 链：summary 缺 → description 命中', async () => {
    mDetail.mockResolvedValue({ ...baseDetail, summary: undefined } as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.functionDetail?.description).toBe('主函数描述');
  });

  it('detail.tags 缺省 → 归一 [] + form tags 空串（L213 右翼）', async () => {
    mDetail.mockResolvedValue({ ...baseDetail, tags: undefined } as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.functionDetail?.tags).toEqual([]);
    expect(result.current.form.getFieldsValue().tags).toBe('');
    expect(result.current.jsonViewData.function?.tags).toEqual([]);
  });

  it('perm res 无 items 键 → Array.isArray 右翼 + 默认 function·invoke 行', async () => {
    mGetPerm.mockResolvedValue({} as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.permForm.getFieldsValue().items).toEqual([
      { resource: 'function', actions: ['invoke'], roles: [] },
    ]);
  });

  it('perm 失败 Error.message 翼 + items 置空', async () => {
    mGetPerm.mockRejectedValue(new Error('perm boom'));
    const { result } = renderDetailHook('fn.main');
    await waitFor(() => expect(result.current.permError).toBe('perm boom'));
    expect(result.current.permForm.getFieldsValue().items).toEqual([]);
    expect(result.current.permLoading).toBe(false);
  });

  it('perm 失败非 Error → intl 兜底文案', async () => {
    mGetPerm.mockRejectedValue('plain');
    const { result } = renderDetailHook('fn.main');
    await waitFor(() => expect(result.current.permError).toBe('加载函数权限失败'));
  });

  it('404 降级：desc 命中 → 运行时形态 + runtimeNotSupported', async () => {
    const err404 = Object.assign(new Error('not found'), { response: { status: 404 } });
    mDetail.mockRejectedValue(err404);
    mDescs.mockResolvedValue([
      {
        id: 'fn.main',
        resource: 'desc-res',
        operation: 'desc-op',
        version: undefined, // → '1.0.0' 缺省翼
      },
    ] as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);

    expect(result.current.functionDetail).toMatchObject({
      id: 'fn.main',
      version: '1.0.0',
      resource: 'desc-res',
      operation: 'desc-op',
      provider: 'runtime',
    });
    expect(result.current.permError).toBe('运行时注册的函数不支持权限管理');
    expect(result.current.permForm.getFieldsValue().items).toEqual([]);
    // 降级链不再拉权限
    expect(mGetPerm).not.toHaveBeenCalled();
  });

  it('404 降级：desc.resource 缺省 → form resource 空串（L283 右翼）', async () => {
    mDetail.mockRejectedValue(Object.assign(new Error('nf'), { response: { status: 404 } }));
    mDescs.mockResolvedValue([{ id: 'fn.main', resource: undefined }] as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);
    expect(result.current.functionDetail?.resource).toBeUndefined();
    expect(result.current.form.getFieldsValue().resource).toBe('');
  });

  it('404 降级：desc 未命中 → message.error 函数不存在', async () => {
    const err400 = Object.assign(new Error('bad'), { response: { status: 400 } });
    mDetail.mockRejectedValue(err400);
    mDescs.mockResolvedValue([{ id: 'other.fn' }] as never);
    const { result } = renderDetailHook('fn.main');
    await waitFor(() => expect(result.current.loading).toBe(false));
    await waitFor(() => expect(mError).toHaveBeenCalledWith('函数不存在'));
    expect(result.current.functionDetail).toBeNull();

    // permForm 从未注入 → validateFields 返 {} → values?.items 右翼 []
    await act(async () => {
      await result.current.handleSavePermissions();
    });
    expect(mUpdatePerm).toHaveBeenCalledWith('fn.main', []);
    expect(mSuccess).toHaveBeenCalledWith('权限已更新');
  });

  it('404 降级：listDescriptors 再炸 → 加载函数详情失败', async () => {
    mDetail.mockRejectedValue(Object.assign(new Error('nf'), { response: { status: 404 } }));
    mDescs.mockRejectedValue(new Error('desc down'));
    renderDetailHook('fn.main');
    await waitFor(() => expect(mError).toHaveBeenCalledWith('加载函数详情失败'));
  });

  it('非 400/404（500/裸 Error）→ 加载函数详情失败', async () => {
    mDetail.mockRejectedValueOnce(Object.assign(new Error('srv'), { response: { status: 500 } }));
    const r1 = renderDetailHook('fn.main');
    await waitFor(() => expect(mError).toHaveBeenCalledWith('加载函数详情失败'));
    r1.unmount();

    mDetail.mockRejectedValueOnce(new Error('plain network'));
    const r2 = renderDetailHook('fn.main');
    await waitFor(() => expect(mError).toHaveBeenCalledWith('加载函数详情失败'));
    r2.unmount();
  });

  it('functionId 缺省 → loadDetail 早退（零请求）', () => {
    const { result } = renderDetailHook();
    expect(result.current.loading).toBe(false);
    expect(mDetail).not.toHaveBeenCalled();
    expect(mDescs).not.toHaveBeenCalled();
  });
});

describe('handlers', () => {
  it('handleSave：tags split/trim/filter + 成功链（setEditing false + loadDetail 重入）', async () => {
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);

    await act(async () => {
      await result.current.handleSave({
        name: '新名',
        description: '新描述',
        resource: 'new-res',
        tags: ' a , b ,, c ',
      });
    });
    expect(mUpdate).toHaveBeenCalledWith('fn.main', {
      name: '新名',
      description: '新描述',
      resource: 'new-res',
      tags: ['a', 'b', 'c'],
    });
    expect(mSuccess).toHaveBeenCalledWith('保存成功');
    expect(result.current.editing).toBe(false);
    // 成功后 loadDetail 重入（getFunctionDetail 二次调用）
    await waitFor(() => expect(mDetail).toHaveBeenCalledTimes(2));
  });

  it('handleSave：tags 缺省 → []；失败 → 保存失败', async () => {
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);

    await act(async () => {
      await result.current.handleSave({ tags: undefined });
    });
    expect(mUpdate).toHaveBeenCalledWith('fn.main', expect.objectContaining({ tags: [] }));

    mUpdate.mockRejectedValueOnce(new Error('save fail'));
    await act(async () => {
      await result.current.handleSave({});
    });
    expect(mError).toHaveBeenCalledWith('保存失败');
  });

  it('handleStatusToggle：enable/disable 双翼 + 失败翼', async () => {
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);

    await act(async () => {
      await result.current.handleStatusToggle(true);
    });
    expect(mEnable).toHaveBeenCalledWith('fn.main');
    expect(mSuccess).toHaveBeenCalledWith('函数已启用');
    await waitFor(() => expect(mDetail).toHaveBeenCalledTimes(2)); // loadDetail 重入

    await act(async () => {
      await result.current.handleStatusToggle(false);
    });
    expect(mDisable).toHaveBeenCalledWith('fn.main');
    expect(mSuccess).toHaveBeenCalledWith('函数已禁用');

    mDisable.mockRejectedValueOnce(new Error('x'));
    await act(async () => {
      await result.current.handleStatusToggle(false);
    });
    expect(mError).toHaveBeenCalledWith('状态更新失败');
  });

  it('handleCopy：成功 → 新 ID 提示 + 跳转；失败 → 复制失败', async () => {
    mCopy.mockResolvedValue({ functionId: 'fn.copy-1', newId: 'n-1' } as never);
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);

    await act(async () => {
      await result.current.handleCopy();
    });
    expect(mSuccess).toHaveBeenCalledWith('复制成功，新函数ID: fn.copy-1');
    expect(mPush).toHaveBeenCalledWith('/functions/fn.copy-1');

    mCopy.mockRejectedValueOnce(new Error('copy fail'));
    await act(async () => {
      await result.current.handleCopy();
    });
    expect(mError).toHaveBeenCalledWith('复制失败');
  });

  it('handleDelete：confirm onOk（成功 → 删除 + 跳目录 / 失败 → 删除失败）', async () => {
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);

    await act(async () => {
      result.current.handleDelete();
    });
    // confirm 直桩不渲染 DOM：配置对象含 onOk，删除在确认前不触发
    expect(mConfirm).toHaveBeenCalledWith(expect.objectContaining({ okType: 'danger' }));
    expect(mDelete).not.toHaveBeenCalled();

    const onOk = mConfirm.mock.calls[0][0].onOk as () => Promise<void>;
    await act(async () => {
      await onOk();
    });
    expect(mDelete).toHaveBeenCalledWith('fn.main');
    expect(mSuccess).toHaveBeenCalledWith('删除成功');
    expect(mPush).toHaveBeenCalledWith('/functions/catalog');

    // 失败翼：onOk 内 deleteFunction 抛错 → 删除失败，不跳转
    mDelete.mockRejectedValueOnce(new Error('del fail'));
    mPush.mockClear();
    await act(async () => {
      await onOk();
    });
    expect(mError).toHaveBeenCalledWith('删除失败');
    expect(mPush).not.toHaveBeenCalled();
  });

  it('handleSavePermissions：成功 / Error.message / 非 Error 兜底', async () => {
    const { result } = renderDetailHook('fn.main');
    await settleMain(result);

    await act(async () => {
      await result.current.handleSavePermissions();
    });
    expect(mUpdatePerm).toHaveBeenCalledWith('fn.main', basePerms.items);
    expect(mSuccess).toHaveBeenCalledWith('权限已更新');
    expect(result.current.permSaving).toBe(false);

    mUpdatePerm.mockRejectedValueOnce(new Error('perm save fail'));
    await act(async () => {
      await result.current.handleSavePermissions();
    });
    expect(mError).toHaveBeenCalledWith('perm save fail');

    mUpdatePerm.mockRejectedValueOnce('plain');
    await act(async () => {
      await result.current.handleSavePermissions();
    });
    expect(mError).toHaveBeenCalledWith('更新失败');
  });

  it('functionId 卫兵：无参 hook 下五 handler 不触 service', async () => {
    const { result } = renderDetailHook();
    await act(async () => {
      await result.current.handleSave({});
      await result.current.handleStatusToggle(true);
      await result.current.handleCopy();
      result.current.handleDelete();
      await result.current.handleSavePermissions();
    });
    expect(mUpdate).not.toHaveBeenCalled();
    expect(mEnable).not.toHaveBeenCalled();
    expect(mCopy).not.toHaveBeenCalled();
    expect(mDelete).not.toHaveBeenCalled();
    expect(mUpdatePerm).not.toHaveBeenCalled();
  });
});

describe('contractDiagnostics', () => {
  it('indexDesc.diagnostics 透传 / 缺省 []', async () => {
    mDescs.mockResolvedValue([
      {
        ...baseIndexDesc,
        diagnostics: [{ code: 'schema_breaking_change', field: 'inputSchema', message: '不兼容' }],
      },
    ] as never);
    const r1 = renderDetailHook('fn.main');
    await settleMain(r1.result);
    expect(r1.result.current.contractDiagnostics).toEqual([
      { code: 'schema_breaking_change', field: 'inputSchema', message: '不兼容' },
    ]);
    r1.unmount();

    mDescs.mockResolvedValue([baseIndexDesc] as never); // 重置：diag 缺省翼
    const r2 = renderDetailHook('fn.main');
    await settleMain(r2.result);
    expect(r2.result.current.contractDiagnostics).toEqual([]);
  });
});
