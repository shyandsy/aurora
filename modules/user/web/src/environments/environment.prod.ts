/**
 * Production environment configuration
 */
export const environment = {
  production: true,
  domain: 'https://user.example.com', // Production domain (update with your actual production domain)
  // 登出 / 续期彻底失败后要跳去的登录入口。prd/eng 走 gate 登录壳(整页导航,会被 forwardAuth
  // 门禁接管)。用它而非 SPA 内部 /login,保证登录 UI 统一在壳子、且触发门禁重新校验。
  loginUrl: '/gate/',
  // 门禁 cookie 名(前端写、Traefik forwardAuth + 后端 GateVerify 读)。必须与后端 user.Config.GateCookie
  // 同名,否则门禁失效。默认 admin_gate;项目若改后端 GateCookie,这里同步改(单一事实来源经 environment 注入)。
  gateCookie: 'admin_gate',
  // user 服务(用户中心:认证/RBAC/microservice token)。相对路径,镜像与域名无关。
  userApiBaseUrl: '/api/user/v1',
  apiTimeout: 10000, // 10 seconds timeout for API requests
  errorMessageAutoHideTime: 5000 // 5 seconds for error message auto hide
};
