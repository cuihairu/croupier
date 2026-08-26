import { request } from '@umijs/max';

// Source: internal/api/config/excel_service.go
export type ExcelCompileResult = {
  key: string;
  version: number;
  sheets: number;
  rows: number;
};

// 在线编译：Univer/自研编辑器快照（{sheets:{name:{cellData}}}）→ 注册 gameplay 版本
export async function compileExcelSnapshot(payload: {
  snapshot: unknown;
  key?: string;
  gameId?: string;
  env?: string;
  message?: string;
}): Promise<ExcelCompileResult> {
  return request('/api/v1/configs/excel/compile', {
    method: 'POST',
    data: payload,
  });
}

// 上传 .xlsx 文件编译注册
export async function importExcelFile(
  file: File,
  meta?: { gameId?: string; env?: string; message?: string },
): Promise<ExcelCompileResult> {
  const form = new FormData();
  form.append('file', file);
  if (meta?.gameId) form.append('gameId', meta.gameId);
  if (meta?.env) form.append('env', meta.env);
  if (meta?.message) form.append('message', meta.message);
  return request('/api/v1/configs/excel/import', {
    method: 'POST',
    data: form,
  });
}
