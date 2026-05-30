import React from 'react';

export const DiffView: React.FC<{ left: string; right: string }> = ({ left, right }) => {
  const l = (left || '').replace(/\r\n/g, '\n').split('\n');
  const r = (right || '').replace(/\r\n/g, '\n').split('\n');
  const rows: { type: 'same' | 'add' | 'del'; l?: string; r?: string }[] = [];
  let i = 0;
  let j = 0;
  const W = 3;

  while (i < l.length || j < r.length) {
    if (i < l.length && j < r.length && l[i] === r[j]) {
      rows.push({ type: 'same', l: l[i], r: r[j] });
      i++;
      j++;
      continue;
    }
    let matched = false;
    for (let k = 1; k <= W; k++) {
      if (j + k < r.length && l[i] === r[j + k]) {
        for (let t = 0; t < k; t++) rows.push({ type: 'add', r: r[j + t] });
        j += k;
        matched = true;
        break;
      }
      if (i + k < l.length && l[i + k] === r[j]) {
        for (let t = 0; t < k; t++) rows.push({ type: 'del', l: l[i + t] });
        i += k;
        matched = true;
        break;
      }
    }
    if (!matched) {
      if (i < l.length) rows.push({ type: 'del', l: l[i++] });
      if (j < r.length) rows.push({ type: 'add', r: r[j++] });
    }
  }

  return (
    <div style={{ display: 'flex', gap: 8 }}>
      <pre
        style={{
          flex: 1,
          margin: 0,
          padding: 8,
          background: '#fafafa',
          border: '1px solid #eee',
          overflow: 'auto',
          maxHeight: 420,
        }}
      >
        {rows.map((row, idx) =>
          row.l !== undefined ? (
            <div key={idx} style={{ background: row.type === 'del' ? '#fff1f0' : undefined }}>
              {row.l}
            </div>
          ) : (
            <div key={idx} />
          ),
        )}
      </pre>
      <pre
        style={{
          flex: 1,
          margin: 0,
          padding: 8,
          background: '#fafafa',
          border: '1px solid #eee',
          overflow: 'auto',
          maxHeight: 420,
        }}
      >
        {rows.map((row, idx) =>
          row.r !== undefined ? (
            <div key={idx} style={{ background: row.type === 'add' ? '#f6ffed' : undefined }}>
              {row.r}
            </div>
          ) : (
            <div key={idx} />
          ),
        )}
      </pre>
    </div>
  );
};

export function langOf(fmt: string): string {
  const f = (fmt || '').toLowerCase();
  if (f === 'json') return 'json';
  if (f === 'yaml' || f === 'yml') return 'yaml';
  if (f === 'xml') return 'xml';
  if (f === 'ini') return 'ini';
  if (f === 'csv') return 'plaintext';
  return 'plaintext';
}

export function hasMonaco(): boolean {
  return false;
}
