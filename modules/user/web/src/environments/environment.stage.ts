/**
 * Stage environment configuration
 */
export const environment = {
  production: false,
  domain: 'https://user-stage.example.com', // Stage environment domain (update with your actual stage domain)
  loginUrl: '/login', // stage 无门禁,登出跳 SPA 内部登录页
  gateCookie: 'admin_gate', // 门禁 cookie 名,须与后端 user.Config.GateCookie 同名
  // user 服务(用户中心:认证/RBAC/microservice token)。
  userApiBaseUrl: 'https://api-stage.example.com/api/user/v1',
  apiTimeout: 10000, // 10 seconds timeout for API requests
  errorMessageAutoHideTime: 5000 // 5 seconds for error message auto hide
};
