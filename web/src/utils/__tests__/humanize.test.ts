/**
 * utils/humanize：humanizeFieldKey 的分词矩阵。
 *
 * schema title 缺失时的表单 label 兜底（todo.md F5）：分隔符归一、
 * camelCase 驼峰切分、连续分隔符折叠、中文原样保留、空串回退原 key。
 */
import { humanizeFieldKey } from '../humanize';

describe('humanizeFieldKey', () => {
  it('snake_case / kebab-case / 点路径 → 空格分词 + 首字母大写', () => {
    expect(humanizeFieldKey('player_name')).toBe('Player Name');
    expect(humanizeFieldKey('player-level')).toBe('Player Level');
    expect(humanizeFieldKey('user.profile.email')).toBe('User Profile Email');
  });

  it('camelCase 驼峰切分（小写数字→大写边界）', () => {
    expect(humanizeFieldKey('createdAt')).toBe('Created At');
    expect(humanizeFieldKey('httpStatus')).toBe('Http Status');
    expect(humanizeFieldKey('itemID2Value')).toBe('Item ID2 Value');
  });

  it('混合分隔符与连续分隔符折叠', () => {
    expect(humanizeFieldKey('a__b--c..d')).toBe('A B C D');
    expect(humanizeFieldKey('  player_name  ')).toBe('Player Name');
  });

  it('无词可分（空串/纯分隔符）回退原 key', () => {
    expect(humanizeFieldKey('')).toBe('');
    expect(humanizeFieldKey('___')).toBe('___');
  });

  it('中文与全大写单词原样保留分词', () => {
    expect(humanizeFieldKey('玩家名称')).toBe('玩家名称');
    expect(humanizeFieldKey('ABC')).toBe('ABC');
  });
});
