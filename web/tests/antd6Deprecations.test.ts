/**
 * antd 6 废弃属性守卫（docs/BUGS.md BUG-005）。
 *
 * 背景：仓库从 antd 5 迁到 antd 6（web/package.json `antd@^6.4.3`，实测安装 6.6.0）。
 * antd 6 对一批旧属性只打 `Warning: [antd: X] \`p\` is deprecated` 而**不破坏行为**，
 * 因此 TypeScript 不会报错、页面看起来也正常，废弃用法可以长期潜伏。实测本地栈
 * （scripts/console-audit.mjs，dev 模式）每次进页面都会在 console 刷警告——其中
 * `Drawer.height` 来自全局布局的 GameSelector，等于每翻一页都刷一次。
 *
 * 本用例把 antd 6 的运行时废弃清单固化成一条回归约束：任何人在 JSX 里重新写回
 * `Alert message=` / `Drawer width=` 这类属性，用例立即失败并指出文件行号。
 *
 * 清单来源不是手抄文档，而是**运行时警告表本身**：扫描
 * `node_modules/antd/es` 下各组件 `.js` 里的 `devUseWarning('<Component>')` 及其后紧跟的
 * `['old', 'new']` 废弃对照表。antd 升级后清单会自动跟随，避免这张表自身腐烂。
 */
import fs from 'node:fs';
import path from 'node:path';

const WEB_ROOT = path.resolve(__dirname, '..');
const ANTD_ES = path.join(WEB_ROOT, 'node_modules', 'antd', 'es');
const SRC_ROOT = path.join(WEB_ROOT, 'src');

/** 收集扫描过程中被跳过的目录（与 codemod / 审计脚本保持一致）。 */
const SKIP_DIRS = new Set(['node_modules', 'dist', '.umi', '__tests__', '__mocks__']);

interface Deprecation {
  component: string;
  deprecated: string;
  replacement: string;
}

/**
 * 从 antd 源码抽取运行时废弃对照表。
 *
 * antd 有三种写法，三种都要认：
 *
 * 1) 对照表 + forEach（多数组件）：
 *      const warning = devUseWarning('Alert');
 *      [['closeText', 'closable.closeIcon'], ['message', 'title']].forEach(...)
 * 2) 直接调用（Spin 等）：
 *      const warning = devUseWarning('Spin');
 *      warning.deprecated(!tip, 'tip', 'description');
 * 3) 对象字面量映射（Select 等）：
 *      const warning = devUseWarning('Select');
 *      const deprecatedProps = { dropdownRender: 'popupRender', bordered: 'variant' };
 *
 * 只收「属性名」级别的废弃：若 deprecatedName 形如 `size="default"`
 * （值级建议，渲染行为正常），不属于 JSX 属性，不纳入守卫。
 */
function collectRuntimeDeprecations(): Deprecation[] {
  const found: Deprecation[] = [];
  const add = (component: string, deprecated: string, replacement: string) => {
    // 过滤值级建议（形如 size="default" / classNames.tip and styles.tip）
    if (!/^[A-Za-z][A-Za-z0-9]*$/.test(deprecated)) return;
    found.push({ component, deprecated, replacement });
  };

  const walk = (dir: string): void => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(full);
        continue;
      }
      if (!entry.name.endsWith('.js')) continue;
      const src = fs.readFileSync(full, 'utf8');
      const re = /devUseWarning\('([A-Za-z]+)'\)/g;
      let m: RegExpExecArray | null;
      while ((m = re.exec(src)) !== null) {
        const component = m[1];
        // 废弃声明紧跟在 devUseWarning 调用之后的同一段代码里
        const tail = src.slice(m.index, m.index + 900);
        // 形式 1：['old', 'new'] 对照表
        const pairRe = /\[\s*'([A-Za-z][A-Za-z0-9]*)'\s*,\s*'([^']+)'\s*\]/g;
        let p: RegExpExecArray | null;
        while ((p = pairRe.exec(tail)) !== null) {
          add(component, p[1], p[2]);
        }
        // 形式 2：warning.deprecated(<cond>, 'old', 'new')
        const callRe = /\.deprecated\(\s*[^,()]+,\s*'([^']+)'\s*,\s*'([^']+)'\s*\)/g;
        let c: RegExpExecArray | null;
        while ((c = callRe.exec(tail)) !== null) {
          add(component, c[1], c[2]);
        }
        // 形式 3：{ old: 'new', ... } 映射。
        // 先整体截出对象字面量，再在体内逐条解析——直接用单条正则会因
        // lastIndex 跳过逗号后的其余条目。
        const objRe = /\{([^{}]*)\}/g;
        let ob: RegExpExecArray | null;
        while ((ob = objRe.exec(tail)) !== null) {
          const entryRe = /([A-Za-z][A-Za-z0-9]*)\s*:\s*'([^']+)'/g;
          let e: RegExpExecArray | null;
          while ((e = entryRe.exec(ob[1])) !== null) {
            add(component, e[1], e[2]);
          }
        }
      }
    }
  };
  walk(ANTD_ES);

  // 去重（同一组件的对照表可能在多个文件里重复声明）
  const seen = new Set<string>();
  return found.filter((d) => {
    const key = `${d.component}.${d.deprecated}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

/** 列出 src 下所有 .tsx 源文件（跳过测试与产物目录）。 */
function collectSourceFiles(): string[] {
  const out: string[] = [];
  const walk = (dir: string): void => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      if (entry.isDirectory()) {
        if (SKIP_DIRS.has(entry.name)) continue;
        walk(path.join(dir, entry.name));
        continue;
      }
      if (entry.name.endsWith('.tsx')) out.push(path.join(dir, entry.name));
    }
  };
  walk(SRC_ROOT);
  return out;
}

/**
 * 返回从 `<Name` 开始的 JSX 开始标签文本。
 *
 * 需要跳过 `{}` 表达式与字符串字面量里的 `>`，否则
 * `<Alert title={a > b} />` 会在表达式中间被误判为标签结束。
 */
function readOpeningTag(src: string, start: number): string {
  let depth = 0;
  let i = start;
  while (i < src.length) {
    const c = src[i];
    if (c === '"' || c === "'" || c === '`') {
      const quote = c;
      i += 1;
      while (i < src.length && src[i] !== quote) {
        if (src[i] === '\\') i += 1;
        i += 1;
      }
      i += 1;
      continue;
    }
    if (c === '{') depth += 1;
    else if (c === '}') depth -= 1;
    else if (c === '>' && depth === 0) return src.slice(start, i);
    i += 1;
  }
  return src.slice(start);
}

interface Violation {
  file: string;
  line: number;
  component: string;
  deprecated: string;
  replacement: string;
}

function findViolations(deprecations: Deprecation[]): Violation[] {
  // component -> [deprecated prop]
  const byComponent = new Map<string, string[]>();
  for (const d of deprecations) {
    const list = byComponent.get(d.component) ?? [];
    list.push(d.deprecated);
    byComponent.set(d.component, list);
  }

  const violations: Violation[] = [];
  for (const file of collectSourceFiles()) {
    const src = fs.readFileSync(file, 'utf8');
    const tagRe = /<([A-Z][A-Za-z0-9_.]*)/g;
    let m: RegExpExecArray | null;
    while ((m = tagRe.exec(src)) !== null) {
      // 只看 antd 组件自身的直接标签；`Foo.Bar` 取最后一段
      const base = m[1].split('.').pop() as string;
      const props = byComponent.get(base);
      if (!props) continue;
      const tag = readOpeningTag(src, m.index);
      for (const prop of props) {
        // JSX 属性位置：前面不能是标识符字符 / `.`，后面紧跟 `=`
        const re = new RegExp(`(?<![A-Za-z0-9_$.])${prop}\\s*=`);
        if (re.test(tag)) {
          violations.push({
            file: path.relative(WEB_ROOT, file),
            line: src.slice(0, m.index).split('\n').length,
            component: base,
            deprecated: prop,
            replacement:
              deprecations.find((d) => d.component === base && d.deprecated === prop)
                ?.replacement ?? '?',
          });
        }
      }
    }
  }
  return violations;
}

describe('antd 6 废弃属性守卫', () => {
  const deprecations = collectRuntimeDeprecations();

  it('能从安装的 antd 中解析出运行时废弃对照表（清单本身没失效）', () => {
    // 抽取失败会让下面的守卫变成"永远通过"的假阴性，必须显式锁住。
    expect(deprecations.length).toBeGreaterThan(50);
    // 抽样校验几个本次实际迁移过的对照项
    const keys = new Set(deprecations.map((d) => `${d.component}.${d.deprecated}`));
    expect(keys).toContain('Alert.message');
    expect(keys).toContain('Drawer.width');
    expect(keys).toContain('Drawer.height');
    expect(keys).toContain('Space.direction');
    expect(keys).toContain('Statistic.valueStyle');
    expect(keys).toContain('Card.bordered');
    expect(keys).toContain('Divider.type');
    // Spin 用的是 warning.deprecated(...) 直接调用形式，抽取器必须也认
    expect(keys).toContain('Spin.tip');
    expect(keys).toContain('Select.dropdownRender');
  });

  it('src 下不存在 antd 6 已废弃的 JSX 属性', () => {
    const violations = findViolations(deprecations);
    if (violations.length > 0) {
      const detail = violations
        .map(
          (v) =>
            `  ${v.file}:${v.line}  <${v.component} ${v.deprecated}=…>  → 改用 ${v.replacement}`,
        )
        .join('\n');
      throw new Error(
        `发现 ${violations.length} 处 antd 6 废弃属性（运行时会在 console 刷 warning）：\n${detail}`,
      );
    }
    expect(violations).toHaveLength(0);
  });

  it('被替换掉的写法确实不在源码里（防守卫自身被注释/字符串绕过）', () => {
    // 这些是本次迁移里最容易被"改回去"的几个，按字面量再确认一次。
    const files = collectSourceFiles();
    const offenders: string[] = [];
    for (const file of files) {
      const src = fs.readFileSync(file, 'utf8');
      const rel = path.relative(WEB_ROOT, file);
      for (const needle of ['addonBefore=', 'addonAfter=', 'valueStyle=']) {
        if (src.includes(needle)) offenders.push(`${rel}: ${needle}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  /**
   * BUG-009：行操作编辑器与常量字段编辑器属批量渲染热路径，不得改回
   * `Space.Compact` 承载标签前缀。
   *
   * antd 官方对 `addonBefore` 的迁移建议是 `Space.Compact`。但本仓库这两个
   * 编辑器会在表格/预览里被大批量实例化，而 `Space.Compact` 会为**每个子项**新建
   * 一次上下文对象（`CompactItem` 的 `useMemo` 依赖是每次渲染新建的 `others`），
   * 上下文值每渲染必变，下游 `Input` 全部被拖进重渲染。实测把两处前缀包上
   * `Space.Compact` 后，jest 全量从 ~145s 涨到 ~207s，且
   * `previewActions.test.tsx` 大面积触发 5s 超时（不同用例轮流失败）。
   *
   * 因此这两处改用未被废弃的 `Input.prefix`——同样能承载标签，且不引入额外组件。
   * 这里做源码级锁定，防止后续"照官方文档改回去"。
   */
  it('热路径编辑器的标签前缀用 Input.prefix，未被 Space.Compact 包裹', () => {
    /**
     * 截取「承载标签的那个元素」所在的源码片段，再断言片段内部不含
     * `Space.Compact`。比整文件匹配精确：文件里包裹 Select 的 `Space.Compact`
     * 是既有写法，与本条无关，不能一并禁掉。
     */
    const sliceBetween = (src: string, from: string, to: string): string | null => {
      const a = src.indexOf(from);
      if (a < 0) return null;
      const b = src.indexOf(to, a);
      if (b < 0) return null;
      return src.slice(a, b);
    };

    /**
     * 去掉注释后再判断——这两个文件里正好有一段解释「为何不用 Space.Compact」
     * 的注释，直接做子串匹配会被注释本身命中。
     */
    const stripComments = (s: string): string =>
      s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');

    const action = stripComments(
      fs.readFileSync(
        path.join(WEB_ROOT, 'src/pages/PageStudio/CompositeEditor/ActionEditor.tsx'),
        'utf8',
      ),
    );
    const actionBlock = sliceBetween(action, '.map((pf) => (', '\n        ))}');
    expect(actionBlock).not.toBeNull();
    // paramFields 的输入框由 prefix 承载标签，且不是 Space.Compact 的子节点
    expect(actionBlock).toMatch(/<Input\b/);
    expect(actionBlock).toMatch(/prefix=\{<span/);
    expect(actionBlock).not.toContain('<Space.Compact');

    const constant = stripComments(
      fs.readFileSync(
        path.join(WEB_ROOT, 'src/pages/PageStudio/CompositeEditor/ConstantFieldsEditor.tsx'),
        'utf8',
      ),
    );
    // 两个常量字段输入框分别以 titleAddon / varNameAddon 作为 prefix
    for (const addon of ['titleAddon', 'varNameAddon']) {
      const idx = constant.indexOf(addon);
      expect(idx).toBeGreaterThan(0);
      // 往前回溯到该 <Input 的开头，确认 prefix 就在这个元素里
      const head = constant.lastIndexOf('<Input', idx);
      const frag = constant.slice(head, idx);
      expect(frag).toMatch(/prefix=\{/);
      expect(frag).not.toContain('<Space.Compact');
    }
  });

  /**
   * 一部分废弃项不是 JSX 属性，而是 `items={[...]}` 对象字面量里的字段
   * （如 `Steps` 的 `items[].description`）。上面的 JSX 属性扫描看不到它们，
   * 这里单独覆盖。
   */
  it('items 数组字面量里没有 antd 6 废弃的字段名', () => {
    // `description` 在 antd 里是 Alert/Descriptions/Timeline 等的合法属性，
    // 只有 Steps 的 items 数组里被改名；因此只对 <Steps …> 的 items 块断言。
    const stepsBlocks: { file: string; block: string }[] = [];
    for (const file of collectSourceFiles()) {
      const src = fs.readFileSync(file, 'utf8');
      let m: RegExpExecArray | null;
      const tagRe = /<Steps\b/g;
      while ((m = tagRe.exec(src)) !== null) {
        const tag = readOpeningTag(src, m.index);
        const open = tag.indexOf('items={[');
        if (open < 0) continue;
        // 从 items={[ 扫到配对的 ]}（Steps 的 items 通常在同一标签内闭合）
        const rest = src.slice(m.index + open + 'items={['.length);
        const close = rest.indexOf(']}');
        if (close < 0) continue;
        stepsBlocks.push({
          file: path.relative(WEB_ROOT, file),
          block: rest.slice(0, close),
        });
      }
    }
    const offenders = stepsBlocks
      .filter((b) => /(^|[{,\s])description\s*:/.test(b.block))
      .map((b) => `${b.file}: Steps items[].description`);
    expect(offenders).toEqual([]);
  });
});
