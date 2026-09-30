/**
 * JWT 解码工具(纯前端,**不校验签名**)。
 *
 * 用途:从当前 access token 里读 claims(尤其 features)做 UI 门控/展示。签名的真伪由后端
 * 每个接口的 JWTAuthMiddleware 校验 —— 前端解出来的 features 只用于「别把用不了的菜单/页面摆出来」,
 * 不是安全边界。用 token 而非 localStorage 的 user 快照,是因为 token 是权威且随刷新自动更新的,
 * 快照会过期/被清空,导致门控失效或误判。
 */

/** 解 JWT payload(第二段 base64url);格式非法/解析失败返回 null。 */
export function decodeJwtPayload(token: string | null | undefined): Record<string, unknown> | null {
  if (!token) {
    return null;
  }
  const parts = token.split('.');
  if (parts.length < 2 || !parts[1]) {
    return null;
  }
  try {
    let b64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const pad = b64.length % 4;
    if (pad) {
      b64 += '='.repeat(4 - pad);
    }
    const json = decodeURIComponent(
      atob(b64)
        .split('')
        .map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join('')
    );
    return JSON.parse(json) as Record<string, unknown>;
  } catch {
    return null;
  }
}

/** 从 access token 权威解出 features 列表;解不出/无该声明返回 []。 */
export function extractFeaturesFromToken(token: string | null | undefined): string[] {
  const payload = decodeJwtPayload(token);
  const f = payload?.['features'];
  return Array.isArray(f) ? (f as unknown[]).filter((x): x is string => typeof x === 'string') : [];
}
