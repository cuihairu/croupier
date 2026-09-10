/** 扩展安装页共享工具。 */
export function formatUnix(ts?: number) {
  if (!ts) return '-';
  const ms = ts > 1e12 ? ts : ts * 1000;
  return new Date(ms).toLocaleString();
}
