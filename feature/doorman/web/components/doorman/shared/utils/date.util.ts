/**
 * 轻量日期格式化,用于规则列表的 created/modified 展示。零外部依赖(方便随组件一起拷)。
 * ⚠️ 默认按**北京时间 UTC+8** 展示;换时区就改下面的 `+ 8 * 60 * 60 * 1000` 偏移。
 */
export function formatDate(dateString: string | null | undefined): string {
  if (!dateString) {
    return '';
  }
  const date = new Date(dateString);
  if (isNaN(date.getTime())) {
    return '';
  }
  const beijing = new Date(date.getTime() + 8 * 60 * 60 * 1000);
  const y = beijing.getUTCFullYear();
  const mo = String(beijing.getUTCMonth() + 1).padStart(2, '0');
  const d = String(beijing.getUTCDate()).padStart(2, '0');
  const h = String(beijing.getUTCHours()).padStart(2, '0');
  const mi = String(beijing.getUTCMinutes()).padStart(2, '0');
  const s = String(beijing.getUTCSeconds()).padStart(2, '0');
  return `${y}-${mo}-${d} ${h}:${mi}:${s}`;
}
