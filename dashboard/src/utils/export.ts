/**
 * Export data as CSV (multi-sheet supported by downloading multiple CSV files).
 *
 * The project currently does not bundle XLSX libraries; callers may still use
 * this function name for historical reasons.
 */
export async function exportToXLSX(fileName: string, sheets: { sheet: string; rows: any[][] }[]) {
  if (!sheets || sheets.length === 0) return;
  const baseName = (fileName || 'export').replace(/\.(xlsx|csv)$/i, '');
  if (sheets.length === 1) {
    exportToCSV(`${baseName}.csv`, sheets[0].rows);
    return;
  }

  // Multi-sheet: trigger multiple downloads (one CSV per sheet).
  for (const s of sheets) {
    const safeSheet =
      String(s.sheet || 'sheet')
        .trim()
        .toLowerCase()
        .replace(/[^a-z0-9._-]+/g, '_')
        .replace(/^_+|_+$/g, '') || 'sheet';
    exportToCSV(`${baseName}_${safeSheet}.csv`, s.rows);
  }
}

export function exportToCSV(fileName: string, rows: any[][]) {
  try {
    const csv = rows
      .map((r) =>
        r
          .map((x) => {
            const s = String(x ?? '');
            return /[",\n]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s;
          })
          .join(','),
      )
      .join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = fileName || 'export.csv';
    a.click();
    URL.revokeObjectURL(url);
  } catch {}
}
