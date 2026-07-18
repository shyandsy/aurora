# Mail Feature(邮件)

基于 `gopkg.in/mail.v2` 的 SMTP 发信,支持纯文本 / HTML / 两者混合。

源码:[feature/mail.go](../../feature/mail.go)、[config/mail.go](../../config/mail.go)

---

## 创建 & 用法

```go
app.AddFeature(feature.NewMailFeature())
```

以接口 `EmailService` 注入([feature/mail.go:43](../../feature/mail.go#L43)):

```go
type EmailService interface {
    SendText(ctx, to []string, subject, body string) error
    SendHTML(ctx, to []string, subject, htmlBody string) error
    Send(ctx, to []string, subject, textBody, htmlBody string) error   // 同时带 text+html
}
```

行为([feature/mail.go:80](../../feature/mail.go#L80) `send`):
- From 头:有 `MAIL_FROM_NAME` 时为 `Name <email>`,否则纯 email;
- text+html 都给 → `text/plain` 正文 + `text/html` alternative;只给一个 → 对应类型;两个都空 → 报错;
- 底层 `dialer.DialAndSend`。

> ⚠️ 接口签名带 `ctx context.Context`,但**实际发信没用到 ctx**(不支持超时/取消)。

## 环境变量 —— 全部强制必填(FromName 除外)

`MailConfig.Validate` 强制校验([config/mail.go:16](../../config/mail.go#L16)),缺任一 → `Setup` 失败、进程不启动:

| Env | 字段 | 必填 | 说明 |
|---|---|---|---|
| `MAIL_SMTP_HOST` | SMTPHost | ✅ | SMTP 主机 |
| `MAIL_SMTP_PORT` | SMTPPort | ✅ | int,1–65535 |
| `MAIL_SMTP_USER` | SMTPUser | ✅ | 用户名 |
| `MAIL_SMTP_PASSWORD` | SMTPPassword | ✅ | 密码 |
| `MAIL_FROM_EMAIL` | FromEmail | ✅ | 发件邮箱 |
| `MAIL_FROM_NAME` | FromName | ❌(omitempty) | 发件人显示名 |

> ⚠️ 因为**强制必填**,即使某服务不真的发信,也得给这些 env 填占位值(否则起不来)。homeserver 就用 `noop.invalid` 之类占位过校验。若希望"可选发信",要改成把这些字段标 `omitempty` 并在发信处判空 —— 当前实现不支持。
