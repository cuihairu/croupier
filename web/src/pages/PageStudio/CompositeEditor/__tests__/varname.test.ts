import {
  assignVarNames,
  baseVarName,
  camelize,
  collectVarNames,
  generateVarName,
  isValidVarName,
  renameVariable,
  rewriteVarRefsInString,
} from '../varname';
import type { PageNode } from '../model';

function n(
  type: string,
  id: string,
  props: Record<string, unknown> = {},
  children?: PageNode[],
): PageNode {
  return { id, type: type as PageNode['type'], props, children };
}

describe('varname: 命名规则', () => {
  it('变量名格式校验：camelCase、字母开头、ASCII', () => {
    expect(isValidVarName('playerListTable')).toBe(true);
    expect(isValidVarName('a')).toBe(true);
    expect(isValidVarName('PlayerList')).toBe(false); // 大写开头
    expect(isValidVarName('player.list')).toBe(false); // 点号
    expect(isValidVarName('玩家表格')).toBe(false); // 中文
    expect(isValidVarName('2fa')).toBe(false); // 数字开头
    expect(isValidVarName('player_list')).toBe(false); // 下划线
    expect(isValidVarName('')).toBe(false);
  });

  it('camelize：分隔符切段 + 首段小写', () => {
    expect(camelize('player')).toBe('player');
    expect(camelize('send mail')).toBe('sendMail');
    expect(camelize('MailSend')).toBe('mailSend');
    expect(camelize('game-config_v2')).toBe('gameConfigV2');
    expect(camelize('发邮件')).toBe(''); // 非 ASCII → 空
    expect(camelize('2fa')).toBe(''); // 数字开头 → 空
    expect(camelize('')).toBe('');
  });

  it('函数组件基名：资源 camelCase + 操作 Camel + 类型后缀', () => {
    expect(baseVarName({ type: 'fnTable', functionId: 'player.list' })).toBe('playerListTable');
    expect(baseVarName({ type: 'fnForm', functionId: 'mail.send' })).toBe('mailSendForm');
    expect(baseVarName({ type: 'fnFields', functionId: 'player.get' })).toBe('playerGetFields');
    expect(baseVarName({ type: 'fnTable', functionId: 'game-config.mail.send' })).toBe(
      'gameConfigMailSendTable',
    );
    // 无函数绑定的回退
    expect(baseVarName({ type: 'fnTable' })).toBe('table');
    expect(baseVarName({ type: 'fnForm' })).toBe('form');
    expect(baseVarName({ type: 'fnFields' })).toBe('fields');
  });

  it('基础组件基名：标题 camelize + 类型后缀，中文标题回退', () => {
    expect(baseVarName({ type: 'modal', title: 'Send Mail' })).toBe('sendMailModal');
    expect(baseVarName({ type: 'modal', title: '发邮件' })).toBe('modal');
    expect(baseVarName({ type: 'button', title: 'Grant Item' })).toBe('grantItemButton');
    expect(baseVarName({ type: 'button' })).toBe('button');
    expect(baseVarName({ type: 'staticForm', title: 'Filter' })).toBe('filterForm');
    expect(baseVarName({ type: 'staticForm' })).toBe('filterForm');
    expect(baseVarName({ type: 'container' })).toBe('container');
    expect(baseVarName({ type: 'text' })).toBe('text');
  });

  it('自动去重：冲突追加最小数字后缀', () => {
    const existing = new Set(['playerListTable', 'playerListTable2', 'playerListTable4']);
    expect(generateVarName({ type: 'fnTable', functionId: 'player.list' }, existing)).toBe(
      'playerListTable3',
    );
    expect(generateVarName({ type: 'modal' }, new Set())).toBe('modal');
    expect(generateVarName({ type: 'modal' }, new Set(['modal', 'modal2']))).toBe('modal3');
  });

  it('collectVarNames：收集树内全部声明 key（含弹窗子级）', () => {
    const tree = [
      n('fnTable', 't1', { sectionKey: 'playerListTable' }),
      n('modal', 'm1', {}, [n('fnForm', 'f1', { sectionKey: 'mailSendForm' })]),
      n('text', 'x1'),
    ];
    expect([...collectVarNames(tree)].sort()).toEqual(['mailSendForm', 'playerListTable']);
  });
});

describe('varname: 引用重写', () => {
  it('表达式段内词边界替换', () => {
    expect(
      rewriteVarRefsInString('{{playerListTable.selectedRow.uid}}', 'playerListTable', 'players'),
    ).toBe('{{players.selectedRow.uid}}');
    // 更长标识符不受影响
    expect(
      rewriteVarRefsInString('{{playerListTable2.data.total}}', 'playerListTable', 'players'),
    ).toBe('{{playerListTable2.data.total}}');
    // 一个串内多处引用
    expect(rewriteVarRefsInString('{{a.x}} 与 {{a.y}}', 'a', 'b')).toBe('{{b.x}} 与 {{b.y}}');
  });

  it('裸引用头部替换（遗留链参数形态）', () => {
    expect(rewriteVarRefsInString('playerListTable.uid', 'playerListTable', 'players')).toBe(
      'players.uid',
    );
    expect(rewriteVarRefsInString('playerListTable', 'playerListTable', 'players')).toBe('players');
    // 非头部/更长标识符不动
    expect(rewriteVarRefsInString('playerListTable2.uid', 'playerListTable', 'players')).toBe(
      'playerListTable2.uid',
    );
    expect(rewriteVarRefsInString('见 playerListTable 输出', 'playerListTable', 'players')).toBe(
      '见 playerListTable 输出',
    );
  });
});

describe('varname: 改名同步', () => {
  const tree: PageNode[] = [
    n('fnTable', 't1', { sectionKey: 'playerListTable', title: '玩家列表' }),
    n('modal', 'm1', { title: '发邮件' }, [
      n('fnForm', 'f1', {
        sectionKey: 'mailSendForm',
        inputAssignments: [
          { param: 'playerId', kind: 'page_state', sourceNodeId: 't1', field: 'uid' },
          { param: 'title', kind: 'literal', value: '{{playerListTable.selectedRow.nickname}}' },
        ],
        chainParams: { uid: 'playerListTable.selectedRow.uid' },
      }),
    ]),
  ];

  it('改名重写节点自身 key 与全树引用（含嵌套 props）', () => {
    const renamed = renameVariable(tree, 'playerListTable', 'playersTable');
    expect(renamed[0].props.sectionKey).toBe('playersTable');
    const form = renamed[1].children![0];
    const assignments = form.props.inputAssignments as Array<Record<string, unknown>>;
    expect(assignments[0].sourceNodeId).toBe('t1'); // 节点 id 引用不受改名影响
    expect(assignments[1].value).toBe('{{playersTable.selectedRow.nickname}}');
    expect((form.props.chainParams as Record<string, string>).uid).toBe(
      'playersTable.selectedRow.uid',
    );
    // 原树不被修改（纯函数）
    expect(tree[0].props.sectionKey).toBe('playerListTable');
  });

  it('非法新名 / 冲突 / 同名 → 原样返回', () => {
    expect(renameVariable(tree, 'playerListTable', '中文名')).toBe(tree);
    expect(renameVariable(tree, 'playerListTable', 'mailSendForm')).toBe(tree); // 冲突
    expect(renameVariable(tree, 'playerListTable', 'playerListTable')).toBe(tree); // 同名
  });
});

describe('assignVarNames（复制/落树语义）', () => {
  it('复制件（未命名）生成新名，不影响原节点与其引用', () => {
    const original: PageNode = {
      id: 'n1',
      type: 'fnTable',
      props: { sectionKey: 'playerListTable', functionId: 'player.list' },
    };
    // clone 已剥离 sectionKey 的副本，内部引用仍指向原变量
    const copy: PageNode = {
      id: 'n2',
      type: 'fnTable',
      props: { functionId: 'player.list', onRowSelected: '{{playerListTable.selectedRow.uid}}' },
    };
    const existing = collectVarNames([original]);
    const [named] = assignVarNames([copy], existing);
    expect(named.props.sectionKey).toBe('playerListTable2');
    // 原节点未受影响
    expect(original.props.sectionKey).toBe('playerListTable');
    // 副本对原变量的引用保持指向原变量（复制语义 = 镜像原行为）
    expect(named.props.onRowSelected).toBe('{{playerListTable.selectedRow.uid}}');
  });

  it('模板子树声明 key 与页面冲突 → 重新生成并重写子树内部引用', () => {
    const pageNode: PageNode = {
      id: 'p1',
      type: 'fnTable',
      props: { sectionKey: 'playerListTable', functionId: 'player.list' },
    };
    const tplNode: PageNode = {
      id: 't1',
      type: 'fnTable',
      props: {
        sectionKey: 'playerListTable',
        functionId: 'player.list',
        onSuccess: {
          kind: 'runBinding',
          target: '',
          params: { uid: '{{playerListTable.selectedRow.uid}}' },
        },
      },
    };
    const existing = collectVarNames([pageNode]);
    const [named] = assignVarNames([tplNode], existing);
    expect(named.props.sectionKey).toBe('playerListTable2');
    // 子树内部引用随树整体重写
    const params = (named.props.onSuccess as { params: Record<string, string> }).params;
    expect(params.uid).toBe('{{playerListTable2.selectedRow.uid}}');
    // 页面原节点引用不受影响
    expect(pageNode.props.sectionKey).toBe('playerListTable');
  });
});
