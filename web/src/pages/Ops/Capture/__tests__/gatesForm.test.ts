/**
 * gatesForm 纯数据层单测：parse 容错 / serialize 恒四段 / 往返一致 /
 * 坏 JSON 抛错 / HH:mm 展示钳制。
 */
import { emptyGatesForm, minutesToHHmm, parseGatesForm, serializeGatesForm } from '../gatesForm';

const FULL = `{
  "reconcile": {
    "rules": [
      {
        "name": "item-ledger",
        "itemTable": "game.items",
        "ledgerTable": "game.ledger",
        "itemKey": "1",
        "ledgerKey": "0",
        "windowSec": 120,
        "severity": "critical",
        "exemptKeys": ["batch-1", "batch-2"],
        "maxPending": 500
      }
    ]
  },
  "cep": {
    "offHours": [
      {
        "name": "night",
        "table": "game.items",
        "ops": ["insert"],
        "windows": [{ "startMin": 0, "endMin": 360 }],
        "tzOffsetMin": 480,
        "severity": "warn"
      }
    ],
    "burst": [
      {
        "name": "gold-drain",
        "table": "game.wallet",
        "ops": [],
        "deltaField": "2",
        "threshold": 10000,
        "mode": "window",
        "windowSec": 60,
        "maxEvents": 100,
        "severity": "critical"
      }
    ],
    "probing": [
      {
        "name": "slow-scrape",
        "table": "game.items",
        "ops": ["update"],
        "keyField": "0",
        "deltaField": "3",
        "smallBelow": 10,
        "count": 5,
        "windowSec": 300,
        "maxTracked": 2000,
        "severity": "warn"
      }
    ]
  },
  "lineage": {
    "rules": [
      {
        "name": "item-writes",
        "table": "game.items",
        "ops": ["insert", "update", "delete"],
        "allowAccounts": ["game-app"],
        "severity": ""
      }
    ]
  },
  "event": { "ops": ["insert", "update"] }
}`;

describe('parseGatesForm', () => {
  it('全字段内容解析为完整表单状态', () => {
    const s = parseGatesForm(FULL);

    expect(s.reconcile).toHaveLength(1);
    expect(s.reconcile[0]).toEqual({
      name: 'item-ledger',
      itemTable: 'game.items',
      ledgerTable: 'game.ledger',
      itemKey: '1',
      ledgerKey: '0',
      windowSec: 120,
      severity: 'critical',
      exemptKeys: ['batch-1', 'batch-2'],
      maxPending: 500,
    });
    expect(s.cepOffHours[0].windows).toEqual([{ startMin: 0, endMin: 360 }]);
    expect(s.cepOffHours[0].tzOffsetMin).toBe(480);
    expect(s.cepBurst[0].mode).toBe('window');
    expect(s.cepProbing[0].count).toBe(5);
    expect(s.lineage[0].allowAccounts).toEqual(['game-app']);
    expect(s.eventOps).toEqual(['insert', 'update']);
  });

  it('缺段/空内容/类型漂移兜底：数组空、数字 0、severity/mode 空', () => {
    expect(parseGatesForm('')).toEqual(emptyGatesForm());
    expect(parseGatesForm('{}')).toEqual(emptyGatesForm());
    expect(parseGatesForm('{"reconcile":{"rules":[]},"cep":{},"lineage":{}}')).toEqual(
      emptyGatesForm(),
    );

    // 类型漂移不炸表单（手写 JSON 容错）
    const drifted = parseGatesForm(
      `{"reconcile":{"rules":[{"name":"x","windowSec":"300","severity":"oops",
        "exemptKeys":"not-array","maxPending":null}]},
        "cep":{"burst":[{"mode":"weird"}]},
        "lineage":{"rules":"not-array"}}`,
    );
    expect(drifted.reconcile[0].windowSec).toBe(0);
    expect(drifted.reconcile[0].severity).toBe('');
    expect(drifted.reconcile[0].exemptKeys).toEqual([]);
    expect(drifted.reconcile[0].maxPending).toBe(0);
    expect(drifted.cepBurst[0].mode).toBe('');
    expect(drifted.lineage).toEqual([]);
  });

  it('坏 JSON 抛错（调用方拦在模式切换外）', () => {
    expect(() => parseGatesForm('{broken')).toThrow();
  });
});

describe('serializeGatesForm', () => {
  it('parse → serialize → parse 往返一致，且恒输出全四段', () => {
    const once = parseGatesForm(FULL);
    const json = serializeGatesForm(once);
    const twice = parseGatesForm(json);

    expect(twice).toEqual(once);
    const root = JSON.parse(json) as {
      reconcile: { rules: unknown[] };
      cep: { offHours: unknown[]; burst: unknown[]; probing: unknown[] };
      lineage: { rules: unknown[] };
      event: { ops: unknown[] };
    };
    expect(Object.keys(root).sort()).toEqual(['cep', 'event', 'lineage', 'reconcile']);
    expect(Object.keys(root.cep).sort()).toEqual(['burst', 'offHours', 'probing']);
  });

  it('空状态序列化为只采不限骨架（与 agent 配置语义一致）', () => {
    const json = JSON.parse(serializeGatesForm(emptyGatesForm())) as {
      reconcile: { rules: unknown[] };
      cep: Record<string, unknown[]>;
      lineage: { rules: unknown[] };
      event: { ops: unknown[] };
    };
    expect(json.reconcile.rules).toEqual([]);
    expect(json.cep.offHours).toEqual([]);
    expect(json.cep.burst).toEqual([]);
    expect(json.cep.probing).toEqual([]);
    expect(json.lineage.rules).toEqual([]);
    expect(json.event.ops).toEqual([]);
  });
});

describe('minutesToHHmm', () => {
  it('分钟转 HH:mm 并钳制到 0-1439', () => {
    expect(minutesToHHmm(0)).toBe('00:00');
    expect(minutesToHHmm(480)).toBe('08:00');
    expect(minutesToHHmm(1439)).toBe('23:59');
    expect(minutesToHHmm(1500)).toBe('23:59');
    expect(minutesToHHmm(-5)).toBe('00:00');
  });
});
