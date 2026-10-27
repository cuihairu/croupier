/** 结构化错误明细列表：展示后端统一错误体的 details（字段路径 → 失败原因）。
 *
 * 预览/保存/发布被服务端拒绝时共用此列表逐条内联呈现原因，
 * 与只展示一条笼统 message 的历史形态相区分（API Response Contract 的 details 面）。
 */
import { extractErrorDetails } from '@/utils/errors';

export default function ErrorDetailList({ error }: { error: unknown }) {
  const details = extractErrorDetails(error);
  if (details.length === 0) return null;
  return (
    <ul style={{ margin: 0, paddingLeft: 18, maxHeight: 260, overflowY: 'auto' }}>
      {details.map((d, i) => (
        <li key={`${d.field}-${i}`}>
          {d.field ? (
            <>
              <code>{d.field}</code>
              {': '}
            </>
          ) : null}
          {d.message}
        </li>
      ))}
    </ul>
  );
}
