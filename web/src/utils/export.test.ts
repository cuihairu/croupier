import { exportToCSV, exportToXLSX } from './export';

// jsdom 未实现 URL.revokeObjectURL：文件级一次补齐并全程 stub（测试文件环境隔离，
// 不做 restore）。createObjectURL 存在但 defineProperty 不可重定义，改用赋值。
type MockedURL = typeof URL & { revokeObjectURL: jest.Mock };
let downloads: string[];

beforeAll(() => {
  jest.spyOn(URL, 'createObjectURL').mockReturnValue('blob:mock');
  (URL as unknown as MockedURL).revokeObjectURL = jest.fn();
});

beforeEach(() => {
  (URL.createObjectURL as jest.Mock).mockClear().mockReturnValue('blob:mock');
  (URL as unknown as MockedURL).revokeObjectURL.mockClear();
  downloads = [];
  jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
    this: HTMLAnchorElement,
  ) {
    downloads.push(this.download);
  });
});

describe('exportToXLSX', () => {
  it('does nothing for empty sheet lists', async () => {
    await exportToXLSX('report', []);
    await exportToXLSX('report', undefined as unknown as []);
    expect(downloads).toEqual([]);
    expect(URL.createObjectURL).not.toHaveBeenCalled();
  });

  it('downloads a single sheet as <base>.csv with the extension stripped', async () => {
    await exportToXLSX('report.xlsx', [{ sheet: 'main', rows: [['a', 1]] }]);

    expect(downloads).toEqual(['report.csv']);
  });

  it('downloads one sanitized CSV per sheet for multi-sheet exports', async () => {
    await exportToXLSX('report', [
      { sheet: 'Player Stats!', rows: [['h']] },
      { sheet: '  ', rows: [['h2']] },
      { sheet: '', rows: [] },
    ]);

    // 'Player Stats!' → 小写 → 非法字符折叠为 _ → 尾部 _ 修剪 → player_stats
    expect(downloads).toEqual(['report_player_stats.csv', 'report_sheet.csv', 'report_sheet.csv']);
    expect(URL.createObjectURL).toHaveBeenCalledTimes(3);
    expect((URL as unknown as MockedURL).revokeObjectURL).toHaveBeenCalledTimes(3);
  });
});

describe('exportToCSV', () => {
  it('builds a CSV blob (quotes/commas/newlines escaped), downloads, and revokes the URL', async () => {
    exportToCSV('rows', [
      ['say "hi"', 'a,b', 'line\nbreak'],
      [null, undefined, true, 0],
    ]);

    expect(downloads).toEqual(['rows']);
    const blob = (URL.createObjectURL as jest.Mock).mock.calls[0][0] as Blob;
    expect(blob.type).toBe('text/csv;charset=utf-8;');
    expect((URL as unknown as MockedURL).revokeObjectURL).toHaveBeenCalledWith('blob:mock');
    // jsdom Blob 无 .text()：经 FileReader 读取序列化内容
    const text = await new Promise<string>((resolve) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.readAsText(blob);
    });
    expect(text).toBe('"say ""hi""","a,b","line\nbreak"\n,,true,0');
  });

  it('falls back to export.csv when the file name is empty', () => {
    exportToCSV('', [['x']]);

    expect(downloads).toEqual(['export.csv']);
  });
});
