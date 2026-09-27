/**
 * 边界与异常数据集 fixture（dev-seed 数据集的前端对位版）。
 *
 * dev-seed（examples/cmd/dev-seed）负责往本地/线上测试库铺数据；本模块把
 * 同一套边界形态提供给 web 单测直接 import，让「写死单条假数据」的用例
 * 升级为数据集驱动。消费方式：整组灌进被测组件的 mock 数据源，断言渲染/
 * 过滤/排序/分页行为（新用例示范见 Admin/Announcements 套件）。
 */

/** 字段边界文本：超长/空串/纯空格/前后空白/emoji/中日韩/RTL/XSS。 */
export const boundaryTexts = {
  /** 200 次重复 ≈ 1000 字符，超过常见 size:255 列与 UI 截断阈值 */
  long: '超长字段'.repeat(200),
  empty: '',
  whitespace: '   ',
  padded: '  前后空白  ',
  emoji: '玩家🎮烽火continu北斗🚀',
  cjk: '日本語Korean중국語简体',
  rtl: 'مرحلةالاختبار',
  /** UI 必须转义展示：文本原样出现、且不得产生 script 元素 */
  xss: `<script>alert('fixture-xss')</script>`,
} as const;

/** 分页阈值组：0/1/恰好一页(20)/±1，与常见默认 pageSize=20 对齐。 */
export function paginationSet(total: number): Array<{ id: number; title: string }> {
  return Array.from({ length: total }, (_, i) => ({
    id: i + 1,
    title: `分页样例-${String(i + 1).padStart(3, '0')}`,
  }));
}

export const paginationThresholds = [0, 1, 19, 20, 21] as const;

/** 时间边界：跨年、未来、同秒组（排序稳定性）。 */
export const timeEdges = {
  lastYear: '2025-12-31T23:59:00+08:00',
  newYear: '2026-01-01T00:01:00+08:00',
  future: '2027-06-01T12:00:00+08:00',
  /** 同 CreatedAt 的三条：同秒排序不应随机抖动 */
  sameSecond: ['同秒排序-A', '同秒排序-B', '同秒排序-C'],
} as const;

/** 悬空/孤儿引用：id 指向不存在的记录，UI 不得崩溃、应优雅降级。 */
export const danglingRefs = {
  playerId: 'ghost-player-404',
  ticketId: 424242,
  announcementId: 88888,
} as const;
