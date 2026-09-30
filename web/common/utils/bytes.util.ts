/**
 * Byte formatting utilities.
 *
 * Backend reports traffic in bytes (int64). These helpers turn a raw byte
 * count into a human-readable string (B / KB / MB / GB / TB), using binary
 * (1024) units to match how xray/most tooling reports traffic.
 */

const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];

/** 1 GiB in bytes (binary, 1024³) — the unit backend quotas are expressed in. */
const GIB = 1024 * 1024 * 1024;

/**
 * Format a byte count into a human-readable string.
 *
 * @param bytes    Raw byte count (int64 from backend). null/undefined/NaN -> '0 B'.
 * @param decimals Number of decimal places for KB and above. Default 2.
 * @returns        e.g. "0 B", "512 B", "1.50 KB", "3.20 GB".
 */
export function formatBytes(bytes: number | null | undefined, decimals = 2): string {
  const value = typeof bytes === 'number' && isFinite(bytes) ? bytes : 0;
  if (value <= 0) {
    return '0 B';
  }
  const k = 1024;
  const i = Math.min(Math.floor(Math.log(value) / Math.log(k)), UNITS.length - 1);
  // Bytes are shown as whole numbers; everything above uses `decimals`.
  const digits = i === 0 ? 0 : Math.max(0, decimals);
  const scaled = value / Math.pow(k, i);
  return `${scaled.toFixed(digits)} ${UNITS[i]}`;
}

/**
 * Format a byte count as a **fixed-unit** GB (GiB, 1024³) number, without the unit suffix.
 *
 * Unlike `formatBytes`, the unit never auto-scales — so a used/quota pair can be
 * rendered side by side ("20.05 / 20.00 GB") and stay visually comparable.
 *
 * @param bytes    Raw byte count (int64 from backend). null/undefined/NaN -> "0.00".
 * @param decimals Number of decimal places. Default 2.
 * @returns        e.g. "0.00", "20.05", "1024.00".
 */
export function formatGiB(bytes: number | null | undefined, decimals = 2): string {
  const value = typeof bytes === 'number' && isFinite(bytes) ? bytes : 0;
  return (Math.max(value, 0) / GIB).toFixed(Math.max(0, decimals));
}

/** 1 MiB / 1 KiB in bytes (binary). */
const MIB = 1024 * 1024;
const KIB = 1024;

/**
 * Format the **used** side of a `used / quota GB` pair with an **adaptive unit**, so tiny
 * usage no longer collapses to "0.00" against a GB quota (most customers use far less than 1 GB).
 *
 *   < 1 MiB  → KB, with unit   e.g. "300.00 KB"
 *   < 1 GiB  → MB, with unit    e.g. "3.20 MB"
 *   ≥ 1 GiB  → GiB **number, no unit** (like `formatGiB`), so it shares the trailing " GB" with the quota.
 *
 * Rendered next to `formatGiB(quota) + " GB"` the pair reads:
 *   "300.00 KB / 5.00 GB" · "3.20 MB / 5.00 GB" · "1.50 / 5.00 GB".
 *
 * @param bytes    Raw byte count. null/undefined/NaN/≤0 → "0.00 KB".
 * @param decimals Decimal places. Default 2.
 */
export function formatUsedGiB(bytes: number | null | undefined, decimals = 2): string {
  const value = typeof bytes === 'number' && isFinite(bytes) ? Math.max(bytes, 0) : 0;
  const d = Math.max(0, decimals);
  if (value >= GIB) {
    return formatGiB(value, d); // 无单位,与后面的 " GB" 尾巴共用
  }
  if (value >= MIB) {
    return `${(value / MIB).toFixed(d)} MB`;
  }
  return `${(value / KIB).toFixed(d)} KB`;
}
