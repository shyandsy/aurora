# tokenguard — token 会话有效性内核(🔌 直接用 Feature,防护三件套之「登录后」)

收口「一枚已签发的 JWT 现在还算不算有效会话」:**撤销(jti 黑名单)+ IP 绑定(防盗用重放)+ token 自描述作用域(scope)**。配合 `jwt` 用——jwt 管"密码学上有效吗",tokenguard 管"这枚有效 token 在会话层面还该被接受吗(登出了没、被踢了没、异地重放没)"。

> 体系定位见专题 **[security-suite](../topics/security-suite.md)**。与前两者正交:loginguard 管登录前、doorman 管每请求风险、tokenguard 管登录后。

## 消费方式
```go
app.AddFeature(tokenguard.NewTokenGuardFeature(denyTTL))   // denyTTL = 黑名单项存活期
// 鉴权中间件里:  Guard tokenguard.Guard `inject:""`  → guard.VerifySession(...)
```
对外只有 `Guard` 契约(校验会话 + 登出/踢设备时把 jti 拉黑)。`SessionTags` / `ScopeOf` / `PublicFeatures` 等辅助按需用。

## 关键点
- **fail-close**:Redis 报错时**拒绝** token,不放行——这是**主级**安全控制(撤销/IP 校验),与限流类(ratelimit/loginguard)的 fail-open 刻意相反。
- **IP 绑定**依赖拿到真实客户端 IP(经反代要正确透传 `ClientIP()`)。
- 只负责"会话层面还有效吗";密码学校验、签发在 `jwt` feature。
