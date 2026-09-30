/**
 * Local/Development environment configuration
 */
export const environment = {
  production: false,
  domain: 'http://localhost:4202', // Local development domain
  loginUrl: '/login', // dev 无门禁,登出跳 SPA 内部登录页
  gateCookie: 'admin_gate', // 门禁 cookie 名,须与后端 user.Config.GateCookie 同名(dev 无门禁,写了无害)
  // user 服务(用户中心:认证/RBAC/microservice token)。本 app 的唯一后端。
  userApiBaseUrl: 'http://localhost:8083/api/user/v1',
  apiTimeout: 10000, // 10 seconds timeout for API requests
  errorMessageAutoHideTime: 5000 // 5 seconds for error message auto hide
};
