/** GameSelector 覆盖：
 * token 守卫（不拉取直接空态）、加载中 spinner、games:changed 事件重载与卸载、
 * 拉取失败清空 scope、响应缺 games 字段；envs/envMeta 两来源 + DEFAULT 表
 * 色点兜底 + 空 env 名过滤；stale scope 回落首个游戏并写 store；受控
 * value/envValue（合法不回调 / 非法回调 fallback、不写内部 state）；UI 换游戏
 * （alias 三级兜底、无 env 目标不持久化、persist 失败吞掉）；UI 换环境
 * （受控跳过内部 state、scope.gameId 缺失不持久化）；header/mobile 变体
 * （抽屉开关、activeAlias/activeEnvLabel 各级兜底）。 */
import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import GameSelector from '.';
import { listMyGames } from '@/services/api';
import type { Game } from '@/services/api';
import { persistMyScope } from '@/services/api/me';
import { getScope, setScope } from '@/stores/scope';

jest.mock('@/services/api', () => ({ listMyGames: jest.fn() }));
jest.mock('@/services/api/me', () => ({ persistMyScope: jest.fn() }));

const mockedList = jest.mocked(listMyGames);
const mockedPersist = jest.mocked(persistMyScope);
const mockGetItem = localStorage.getItem as jest.MockedFunction<typeof localStorage.getItem>;
type GamesResp = Awaited<ReturnType<typeof listMyGames>>;

/** 控制登录态：token 存在与否决定 canListGames */
const setToken = (token: string | null) => {
  mockGetItem.mockReset();
  mockGetItem.mockImplementation((key: string) => (key === 'token' ? token : null));
};

const resetScope = () =>
  setScope({ gameId: undefined, env: undefined }, { persist: false, emit: false });

/** 打开第 index 个 Select 下拉（下拉渲染在 body portal） */
const openSelect = (index: number) => {
  fireEvent.mouseDown(screen.getAllByRole('combobox')[index]);
};

const clickOption = async (text: string) => {
  const option = await waitFor(() => {
    const hit = Array.from(document.querySelectorAll('.ant-select-item-option')).find((o) =>
      o.textContent?.includes(text),
    );
    if (!hit) throw new Error(`选项 ${text} 未渲染`);
    return hit as HTMLElement;
  });
  fireEvent.click(option);
};

beforeEach(() => {
  jest.clearAllMocks();
  setToken('tok');
  resetScope();
  mockedPersist.mockReset();
  mockedPersist.mockResolvedValue(undefined);
});

describe('GameSelector：加载与守卫', () => {
  it('无 token：不拉取，直接空态（游戏 Empty + 环境提示）', async () => {
    setToken(null);
    render(<GameSelector />);

    expect(mockedList).not.toHaveBeenCalled();
    expect(await screen.findByText('暂无游戏')).toBeInTheDocument();
    expect(screen.getByText('未配置可用环境')).toBeInTheDocument();
  });

  it('加载中显示 spinner，完成后出现双 Select', async () => {
    let resolveLoad!: (v: GamesResp) => void;
    mockedList.mockReturnValue(
      new Promise<GamesResp>((resolve) => {
        resolveLoad = resolve;
      }),
    );
    render(<GameSelector />);

    expect(await screen.findByText('加载中')).toBeInTheDocument();
    resolveLoad({ games: [{ name: 'g1', envs: ['prod'] }] });
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));
  });

  it('games:changed 事件触发重载；卸载后不再响应', async () => {
    mockedList.mockResolvedValue({ games: [{ name: 'g1', envs: ['prod'] }] });
    const { unmount } = render(<GameSelector />);
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(1));

    window.dispatchEvent(new Event('games:changed'));
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(2));

    unmount();
    window.dispatchEvent(new Event('games:changed'));
    expect(mockedList).toHaveBeenCalledTimes(2);
  });

  it('拉取失败：清空游戏与本地 scope', async () => {
    const errSpy = jest.spyOn(console, 'error').mockImplementation(() => undefined);
    setScope({ gameId: 'g1', env: 'prod' }, { persist: false, emit: false });
    mockedList.mockRejectedValue(new Error('net-down'));
    render(<GameSelector />);

    expect(await screen.findByText('暂无游戏')).toBeInTheDocument();
    expect(getScope().gameId).toBeUndefined();
    errSpy.mockRestore();
  });

  it('响应缺 games 字段：按空列表处理', async () => {
    mockedList.mockResolvedValueOnce(undefined as unknown as GamesResp);
    render(<GameSelector />);

    expect(await screen.findByText('暂无游戏')).toBeInTheDocument();
    expect(mockedList).toHaveBeenCalledTimes(1);
  });
});

describe('GameSelector：环境来源与兜底', () => {
  it('envMeta 来源：DEFAULT 表色点兜底、自定义色、不命中用 FALLBACK、空 env 名过滤', async () => {
    mockedList.mockResolvedValue({
      games: [
        {
          name: 'g1',
          envMeta: [
            { env: 'dev' },
            { env: 'biz-env', color: '#123456' },
            { env: 'custom-env' },
            { env: '' },
          ],
        },
      ],
    });
    render(<GameSelector />);
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));

    openSelect(1);
    const options = await waitFor(() => {
      const nodes = Array.from(document.querySelectorAll('.ant-select-item-option'));
      if (nodes.length === 0) throw new Error('env 选项未渲染');
      return nodes as HTMLElement[];
    });
    // 空 env 名被过滤：4 项输入 → 3 个选项
    expect(options.length).toBe(3);

    const dotOf = (text: string) => {
      const opt = options.find((o) => o.textContent?.includes(text));
      return (opt?.querySelector('span[style]') as HTMLElement).style.backgroundColor;
    };
    // dev 命中 DEFAULT_ENV_OPTIONS → #1677ff；biz-env 自带色；custom-env 兜底灰
    expect(dotOf('DEV')).toBe('rgb(22, 119, 255)');
    expect(dotOf('BIZ-ENV')).toBe('rgb(18, 52, 86)');
    expect(dotOf('CUSTOM-ENV')).toBe('rgb(140, 140, 140)');
  });

  it('envs 字符串数组来源：映射为环境选项', async () => {
    mockedList.mockResolvedValue({ games: [{ name: 'g1', envs: ['prod'] }] });
    render(<GameSelector />);
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));

    openSelect(1);
    await clickOption('PROD');
    // 选择即写入 scope（gameId 已由回落 effect 写入）
    await waitFor(() => expect(getScope().env).toBe('prod'));
  });

  it('无名游戏：不回落、不回调、环境提示空态', async () => {
    mockedList.mockResolvedValue({ games: [{} as Game] });
    const onChange = jest.fn();
    render(<GameSelector onChange={onChange} />);

    expect(await screen.findByText('未配置可用环境')).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
    expect(getScope().gameId).toBeUndefined();
  });
});

describe('GameSelector：非受控回落与 scope 写入', () => {
  it('stale scope 回落首个游戏：setScope + onEnvChange(fallback env)', async () => {
    setScope({ gameId: 'stale', env: 'stale-env' }, { persist: false, emit: false });
    mockedList.mockResolvedValue({
      games: [
        { name: 'g1', aliasName: 'A1', envs: ['prod', 'dev'] },
        { name: 'g2', envs: ['stage'] },
      ],
    });
    const onEnvChange = jest.fn();
    render(<GameSelector onEnvChange={onEnvChange} />);

    await waitFor(() => expect(getScope().gameId).toBe('g1'));
    expect(getScope().env).toBe('prod');
    expect(onEnvChange).toHaveBeenCalledWith('prod');
    // 选中项 label 显示别名
    expect(screen.getByText('A1')).toBeInTheDocument();
  });
});

describe('GameSelector：受控模式', () => {
  const games: Game[] = [
    { name: 'g1', envMeta: [{ env: 'prod' }, { env: 'dev' }] },
    { name: 'g2', envMeta: [{ env: 'stage' }] },
  ];

  it('合法受控值：不回调、不写 scope', async () => {
    mockedList.mockResolvedValue({ games });
    const onChange = jest.fn();
    const onEnvChange = jest.fn();
    const { rerender } = render(
      <GameSelector value="g1" envValue="prod" onChange={onChange} onEnvChange={onEnvChange} />,
    );
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));
    expect(onChange).not.toHaveBeenCalled();
    expect(onEnvChange).not.toHaveBeenCalled();
    expect(getScope().gameId).toBeUndefined();

    // envValue 换成不存在的环境 → 仅回调 fallback，不写内部 state/scope
    rerender(
      <GameSelector
        value="g1"
        envValue="ghost-env"
        onChange={onChange}
        onEnvChange={onEnvChange}
      />,
    );
    await waitFor(() => expect(onEnvChange).toHaveBeenCalledWith('prod'));
    expect(getScope().env).toBeUndefined();
  });

  it('受控 value 不在游戏列表：回调 fallback，不写 scope', async () => {
    mockedList.mockResolvedValue({ games });
    const onChange = jest.fn();
    render(<GameSelector value="ghost" envValue="prod" onChange={onChange} />);

    await waitFor(() => expect(onChange).toHaveBeenCalledWith('g1'));
    expect(getScope().gameId).toBeUndefined();
  });

  it('受控游戏下 UI 换游戏：跳过内部 state，仍写 scope 并持久化；父组件跟进 value 后 env 回调 fallback', async () => {
    mockedList.mockResolvedValue({ games });
    const onChange = jest.fn();
    const onEnvChange = jest.fn();
    const { rerender } = render(
      <GameSelector value="g1" envValue="prod" onChange={onChange} onEnvChange={onEnvChange} />,
    );
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));

    openSelect(0);
    await clickOption('g2');
    await waitFor(() => expect(onChange).toHaveBeenCalledWith('g2'));
    expect(getScope()).toMatchObject({ gameId: 'g2', env: 'stage' });
    expect(mockedPersist).toHaveBeenCalledWith('g2', 'stage');
    // 受控模式下 activeGame 跟随 value：父组件跟进 onChange 把 value 切到 g2
    rerender(
      <GameSelector value="g2" envValue="prod" onChange={onChange} onEnvChange={onEnvChange} />,
    );
    // g2 只有 stage，envValue=prod 不在选项 → 回调 env fallback
    await waitFor(() => expect(onEnvChange).toHaveBeenCalledWith('stage'));
  });

  it('受控环境下 UI 换环境：跳过内部 state，scope.gameId 缺失不持久化', async () => {
    mockedList.mockResolvedValue({ games });
    const onEnvChange = jest.fn();
    render(<GameSelector value="g1" envValue="prod" onEnvChange={onEnvChange} />);
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));

    openSelect(1);
    await clickOption('DEV');
    await waitFor(() => expect(onEnvChange).toHaveBeenCalledWith('dev'));
    expect(getScope().env).toBe('dev');
    expect(mockedPersist).not.toHaveBeenCalled();
  });
});

describe('GameSelector：非受控 UI 切换', () => {
  it('换游戏（displayName 兜底）：写 scope + 持久化（失败被吞）', async () => {
    mockedList.mockResolvedValue({
      games: [
        { name: 'g1', aliasName: 'A1', envs: ['prod', 'dev'] },
        { name: 'g2', displayName: 'G2 显示名', envs: ['stage'] },
      ],
    });
    mockedPersist.mockRejectedValueOnce(new Error('persist-down'));
    const onChange = jest.fn();
    render(<GameSelector onChange={onChange} />);
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));

    openSelect(0);
    await clickOption('G2 显示名');
    await waitFor(() => expect(onChange).toHaveBeenCalledWith('g2'));
    expect(getScope()).toMatchObject({ gameId: 'g2', env: 'stage' });
    expect(mockedPersist).toHaveBeenCalledWith('g2', 'stage');
    // 选中后 label 回退 displayName（下拉选项与选中 label 各一处）
    expect(screen.getAllByText('G2 显示名').length).toBeGreaterThan(0);
  });

  it('换到无环境游戏：不持久化、清空 scope、环境提示空态', async () => {
    mockedList.mockResolvedValue({
      games: [
        { name: 'g1', envs: ['prod'] },
        { name: 'g2', envMeta: [], envs: [] },
      ],
    });
    const onChange = jest.fn();
    render(<GameSelector onChange={onChange} />);
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));

    openSelect(0);
    await clickOption('g2');
    await waitFor(() => expect(onChange).toHaveBeenCalledWith('g2'));
    expect(mockedPersist).not.toHaveBeenCalled();
    expect(screen.getByText('未配置可用环境')).toBeInTheDocument();
    await waitFor(() => expect(getScope().gameId).toBeUndefined());
  });

  it('换环境：写 scope + 持久化（失败被吞）', async () => {
    mockedList.mockResolvedValue({ games: [{ name: 'g1', envs: ['prod', 'dev'] }] });
    mockedPersist.mockRejectedValueOnce(new Error('env-persist-down'));
    const onEnvChange = jest.fn();
    render(<GameSelector onEnvChange={onEnvChange} />);
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));

    openSelect(1);
    await clickOption('DEV');
    await waitFor(() => expect(onEnvChange).toHaveBeenCalledWith('dev'));
    expect(getScope().env).toBe('dev');
    expect(mockedPersist).toHaveBeenCalledWith('g1', 'dev');
  });

  it('name-only 游戏：别名兜底为 name', async () => {
    mockedList.mockResolvedValue({ games: [{ name: 'solo', envs: ['prod'] }] });
    render(<GameSelector />);
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));
    expect(screen.getAllByText('solo').length).toBeGreaterThan(0);
  });
});

describe('GameSelector：变体渲染', () => {
  it('header 变体：紧凑面板双 Select', async () => {
    mockedList.mockResolvedValue({ games: [{ name: 'g1', envs: ['prod'] }] });
    render(<GameSelector variant="header" />);
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));
  });

  it('mobile 变体：触发按钮显示别名/环境，抽屉可开关', async () => {
    mockedList.mockResolvedValue({
      games: [{ name: 'g1', aliasName: 'A1', envs: ['prod'] }],
    });
    render(<GameSelector variant="mobile" />);
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(1));

    const trigger = screen.getByRole('button');
    await waitFor(() => expect(trigger.textContent).toContain('A1'));
    expect(trigger.textContent).toContain('PROD');

    fireEvent.click(trigger);
    await waitFor(() => expect(document.querySelector('.ant-drawer-open')).not.toBeNull());
    expect(screen.getByText('当前游戏')).toBeInTheDocument();

    fireEvent.click(document.querySelector('.ant-drawer-close') as HTMLElement);
    await waitFor(() => expect(document.querySelector('.ant-drawer-open')).toBeNull());
  });

  it('mobile 变体：envValue 不在选项 → 大写兜底标签', async () => {
    mockedList.mockResolvedValue({ games: [{ name: 'g1', envs: ['prod'] }] });
    const onEnvChange = jest.fn();
    render(
      <GameSelector variant="mobile" value="g1" envValue="legacy" onEnvChange={onEnvChange} />,
    );

    await waitFor(() => expect(onEnvChange).toHaveBeenCalledWith('prod'));
    expect(screen.getByRole('button').textContent).toContain('LEGACY');
  });

  it('mobile 变体：无游戏 → 「未选择游戏」与 ENV 兜底', async () => {
    mockedList.mockResolvedValue({ games: [] });
    render(<GameSelector variant="mobile" />);

    expect(await screen.findByText('未选择游戏')).toBeInTheDocument();
    expect(screen.getByText('ENV')).toBeInTheDocument();
  });
});

describe('GameSelector：envMeta 形态兼容与无名游戏', () => {
  it('envMeta 传入 EnvOption 形态（value/color）：env 名取 value、色点直接生效', async () => {
    // buildEnvOptions 兼容 { value, color } 形态（GameEnvMeta 无 env 字段时回退 value）
    mockedList.mockResolvedValue({
      games: [
        {
          name: 'g1',
          envMeta: [{ value: 'biz-opt', color: '#123456' }],
        } as unknown as Game,
      ],
    });
    render(<GameSelector />);
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(2));

    openSelect(1);
    const option = await waitFor(() => {
      const hit = Array.from(document.querySelectorAll('.ant-select-item-option')).find((o) =>
        o.textContent?.includes('BIZ-OPT'),
      );
      if (!hit) throw new Error('env 选项未渲染');
      return hit as HTMLElement;
    });
    const dot = option.querySelector('span[style]') as HTMLElement;
    expect(dot.style.backgroundColor).toBe('rgb(18, 52, 86)');
  });

  it('无名游戏选项点击：value 为 undefined 时 !next 守卫直接返回，不写 scope', async () => {
    mockedList.mockResolvedValue({ games: [{} as Game] });
    render(<GameSelector />);
    // 无名游戏无 env 元数据：仅游戏下拉渲染，环境位降级为「未配置可用环境」
    await waitFor(() => expect(screen.getAllByRole('combobox').length).toBe(1));
    expect(await screen.findByText('未配置可用环境')).toBeInTheDocument();

    openSelect(0);
    const option = await waitFor(() => {
      const nodes = document.querySelectorAll('.ant-select-item-option');
      if (nodes.length === 0) throw new Error('游戏选项未渲染');
      return nodes[0] as HTMLElement;
    });
    fireEvent.click(option);
    await act(async () => {});
    // 守卫返回：scope 不写入、不持久化、面板不崩
    expect(getScope().gameId).toBeUndefined();
    expect(mockedPersist).not.toHaveBeenCalled();
  });
});
