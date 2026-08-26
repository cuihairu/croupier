import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  App,
  Button,
  Card,
  Input,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  Upload,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { CloudUploadOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import * as XLSX from 'xlsx';
import {
  compileExcelSnapshot,
  importExcelFile,
  type ExcelCompileResult,
} from '@/services/api/excelConfig';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

// 单元格值：数字/布尔/字符串（与后端 coerceCell 语义一致）
type CellValue = string | number | boolean;
type SheetRows = CellValue[][];

type Sheet = {
  name: string;
  // 首行字段名、可选第二行 #type 类型行
  rows: SheetRows;
};

export default function ExcelConfigPage() {
  const { message } = App.useApp();
  const [sheets, setSheets] = useState<Sheet[]>([]);
  const [active, setActive] = useState(0);
  const [configKey, setConfigKey] = useState('excel.workbook');
  const [commitMessage, setCommitMessage] = useState('');
  const [saving, setSaving] = useState(false);
  const [lastResult, setLastResult] = useState<ExcelCompileResult | null>(null);

  const current = sheets[active];

  const loadLocalDraft = useCallback(() => {
    try {
      const raw = localStorage.getItem('excel-config-draft');
      if (raw) {
        const parsed = JSON.parse(raw) as Sheet[];
        if (Array.isArray(parsed) && parsed.length > 0) {
          setSheets(parsed);
          return;
        }
      }
    } catch {
      /* fallthrough */
    }
    setSheets([{ name: 'Sheet1', rows: [['id', 'name', 'value']] }]);
  }, []);

  useEffect(() => {
    loadLocalDraft();
  }, [loadLocalDraft]);

  const persistDraft = useCallback((next: Sheet[]) => {
    localStorage.setItem('excel-config-draft', JSON.stringify(next));
  }, []);

  const updateRows = (rows: SheetRows) => {
    const next = sheets.map((s, i) => (i === active ? { ...s, rows } : s));
    setSheets(next);
    persistDraft(next);
  };

  const setCell = (row: number, col: number, value: string) => {
    const rows = current.rows.map((r) => [...r]);
    while (rows.length <= row) rows.push(new Array(col + 1).fill(''));
    while (rows[row].length <= col) rows[row].push('');
    // 数字自动转 number，与后端未标注类型的 coerce 一致
    let typed: CellValue = value;
    if (/^-?\d+$/.test(value)) typed = Number(value);
    else if (/^-?\d+\.\d+$/.test(value)) typed = Number(value);
    rows[row][col] = typed;
    updateRows(rows);
  };

  const addRow = () => {
    const width = current.rows[0]?.length || 1;
    updateRows([...current.rows, new Array(width).fill('')]);
  };

  const addSheet = () => {
    const name = `Sheet${sheets.length + 1}`;
    const next = [...sheets, { name, rows: [['id']] }];
    setSheets(next);
    persistDraft(next);
    setActive(next.length - 1);
  };

  // 导入 .xlsx：前端 SheetJS 解析为表格草稿（服务端也有同款编译端点）
  const onImportXlsx = async (file: File) => {
    try {
      const buf = await file.arrayBuffer();
      const wb = XLSX.read(buf, { type: 'array' });
      const parsed: Sheet[] = wb.SheetNames.map((name) => ({
        name,
        rows: XLSX.utils.sheet_to_json<CellValue[]>(wb.Sheets[name], {
          header: 1,
          raw: true,
          defval: '',
        }) as CellValue[][],
      }));
      if (parsed.length === 0) {
        message.warning('文件没有 sheet');
        return false;
      }
      setSheets(parsed);
      setActive(0);
      persistDraft(parsed);
      message.success(`已导入 ${parsed.length} 个 sheet（草稿，保存后注册新版本）`);
    } catch (error) {
      message.error(extractErrorMessage(error, '解析失败'));
    }
    return false;
  };

  const exportXlsx = () => {
    const wb = XLSX.utils.book_new();
    for (const sheet of sheets) {
      XLSX.utils.book_append_sheet(wb, XLSX.utils.aoa_to_sheet(sheet.rows), sheet.name);
    }
    XLSX.writeFile(wb, `${configKey || 'excel-config'}.xlsx`);
  };

  // 保存：构造与后端编译器同构的快照（cellData 稀疏格式）
  const buildSnapshot = () => ({
    sheets: Object.fromEntries(
      sheets.map((sheet) => {
        const cellData: Record<string, Record<string, { v: CellValue }>> = {};
        sheet.rows.forEach((row, ri) => {
          row.forEach((cell, ci) => {
            if (cell !== '' && cell !== null && cell !== undefined) {
              cellData[String(ri)] = cellData[String(ri)] || {};
              cellData[String(ri)][String(ci)] = { v: cell };
            }
          });
        });
        return [sheet.name, { cellData }];
      }),
    ),
  });

  const save = async () => {
    setSaving(true);
    try {
      const result = await compileExcelSnapshot({
        snapshot: buildSnapshot(),
        key: configKey,
        message: commitMessage || undefined,
      });
      setLastResult(result);
      message.success(`已注册版本 v${result.version}（${result.sheets} 表 / ${result.rows} 行）`);
      setCommitMessage('');
    } catch (error) {
      message.error(extractErrorMessage(error, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const uploadToServer = async (file: File) => {
    try {
      const result = await importExcelFile(file, { message: commitMessage || undefined });
      setLastResult(result);
      message.success(`服务端编译完成：v${result.version}（${result.rows} 行）`);
    } catch (error) {
      message.error(extractErrorMessage(error, '上传编译失败'));
    }
    return false;
  };

  const columns = useMemo(() => {
    const width = Math.max(
      current?.rows[0]?.length || 0,
      ...((current?.rows || []) as CellValue[][]).map((r) => r.length),
      1,
    );
    return Array.from({ length: width }, (_, ci) => ({
      title:
        active === 0 && ci === 0 ? (
          <Space>
            <Text strong>字段 / 数据</Text>
          </Space>
        ) : (
          <Text type="secondary">列 {ci + 1}</Text>
        ),
      dataIndex: ci,
      width: 160,
      render: (_: unknown, row: CellValue[]) => {
        const ri = (current?.rows || []).indexOf(row);
        const isHeader = ri === 0;
        const isTypeRow = ri === 1 && String(row[0] || '').startsWith('#');
        if (isHeader) {
          return (
            <Input
              size="small"
              variant="borderless"
              value={String(row[ci] ?? '')}
              onChange={(e) => setCell(0, ci, e.target.value)}
              placeholder="字段名"
            />
          );
        }
        if (isTypeRow) {
          return (
            <Select
              size="small"
              variant="borderless"
              value={ci === 0 ? String(row[ci]) : String(row[ci] || '') || undefined}
              onChange={(v) => setCell(1, ci, v)}
              placeholder="类型"
              allowClear
              style={{ width: '100%' }}
              disabled={ci === 0}
              options={
                ci === 0
                  ? [{ label: String(row[0]), value: String(row[0]) }]
                  : ['int', 'string', 'float', 'bool'].map((t) => ({ label: t, value: t }))
              }
            />
          );
        }
        return (
          <Input
            size="small"
            variant="borderless"
            value={String(row[ci] ?? '')}
            onChange={(e) => setCell(ri, ci, e.target.value)}
          />
        );
      },
    }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current, active]);

  return (
    <PageContainer>
      <Card
        title="表格配置（Excel 在线编译）"
        extra={
          <Space wrap>
            <Button icon={<ReloadOutlined />} onClick={loadLocalDraft}>
              重置草稿
            </Button>
            <Upload accept=".xlsx" showUploadList={false} beforeUpload={onImportXlsx}>
              <Button>导入 .xlsx 为草稿</Button>
            </Upload>
            <Button onClick={exportXlsx}>导出 .xlsx</Button>
            <Upload accept=".xlsx" showUploadList={false} beforeUpload={uploadToServer}>
              <Button icon={<CloudUploadOutlined />}>服务端编译上传</Button>
            </Upload>
            <Popconfirm
              title="注册新版本并热更下发？"
              description="保存会生成新的 gameplay 配置版本，游戏服将收到变更通知。"
              onConfirm={save}
            >
              <Button type="primary" icon={<SaveOutlined />} loading={saving}>
                保存并发布
              </Button>
            </Popconfirm>
          </Space>
        }
      >
        <Space wrap style={{ marginBottom: 12 }}>
          <Input
            placeholder="配置 key（如 shop.items）"
            value={configKey}
            onChange={(e) => setConfigKey(e.target.value)}
            style={{ width: 220 }}
          />
          <Input
            placeholder="版本说明（可选）"
            value={commitMessage}
            onChange={(e) => setCommitMessage(e.target.value)}
            style={{ width: 260 }}
          />
          {sheets.map((sheet, i) => (
            <Tag.CheckableTag key={sheet.name} checked={i === active} onChange={() => setActive(i)}>
              {sheet.name}
            </Tag.CheckableTag>
          ))}
          <Button size="small" onClick={addSheet}>
            + sheet
          </Button>
          {lastResult ? (
            <Tag color="green">
              最新版本 v{lastResult.version}（{lastResult.sheets} 表 / {lastResult.rows} 行）
            </Tag>
          ) : null}
        </Space>

        <Table
          size="small"
          rowKey={(_, i) => String(i)}
          dataSource={(current?.rows || []) as unknown as CellValue[][]}
          columns={columns as never}
          pagination={false}
          scroll={{ x: 'max-content' }}
          bordered
        />
        <Space style={{ marginTop: 12 }}>
          <Button onClick={addRow}>+ 行</Button>
          <Text type="secondary">
            约定：首行=字段名；可选第二行首格以 #
            开头=类型行（int/string/float/bool，逐列对齐）；空行忽略。
            草稿自动存本地，保存后在「配置版本」中可查看与回滚。
          </Text>
        </Space>
      </Card>
    </PageContainer>
  );
}
