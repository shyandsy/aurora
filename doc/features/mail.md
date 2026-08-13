# Mail(邮件)

供应商无关的发信抽象。调用方只依赖 `Mailer.Send(ctx, Message)`,用**自己的配置来源**(env / 数据库 / 临时)构造具体 `Mailer` —— 换供应商、换授权方式、换凭据来源都不动调用点。

包:`github.com/shyandsy/aurora/feature/mail`
源码:[mailer.go](../../feature/mail/mailer.go)(接口/Message)、[smtp.go](../../feature/mail/smtp.go)(SMTP 供应商)、[auth.go](../../feature/mail/auth.go)(授权)

> ⚠️ 这**不是**一个自动注册的 `contracts.Features`(不走 `AddFeature`、不读 env)。它是一个库,由 app 按需构造。旧的 `feature.NewMailFeature` / `EmailService` / `config.MailConfig`(从 `MAIL_SMTP_*` 读配置)已移除,见文末「从旧版迁移」。

---

## 核心

```go
type Message struct {
    From    string   // 可选;为空则用供应商默认 From(SMTP 见 WithFrom)
    To, Cc, Bcc []string
    Subject string
    Text    string   // 纯文本正文
    HTML    string   // 可选 HTML 正文
}

// 调用方依赖这个;换实现(供应商/授权)都不动调用点。
type Mailer interface {
    Send(ctx context.Context, msg Message) error
}
```

**内容规则**:只 `Text` → `text/plain`;只 `HTML` → `text/html`;**两者都给 → `multipart/alternative`**(纯文本回退 + HTML);都空 → 报错。`From` 解析后为空也报错(避免发出去被服务器拒)。

---

## SMTP 供应商

```go
m := mail.NewSMTP("smtp.gmail.com", 587,
    mail.WithFrom("me@gmail.com"),
    mail.WithAuth(mail.BasicAuth("me@gmail.com", "<app-password>")),
    mail.WithTimeout(15*time.Second),   // 可选,默认 10s
)

err := m.Send(ctx, mail.Message{
    To:      []string{"user@example.com"},
    Subject: "Hello",
    Text:    "plain body",
    HTML:    "<b>html body</b>",
})
```

- `NewSMTP(host, port, ...SMTPOption)`:host+port 必填;`WithFrom` / `WithAuth` / `WithTimeout` / `WithEncryption` 为可选项。
- 加密方式:`WithEncryption` 显式指定,不设 = **自动**(`EncryptionAuto`,零值):`465` = 隐式 TLS(SSL),其余端口机会式 STARTTLS(服务器不支持则明文降级)。显式取值:
  - `EncryptionNone` —— 明文,禁用 STARTTLS;
  - `EncryptionStartTLS` —— 强制 STARTTLS(不支持即报错);
  - `EncryptionSSL` —— 强制隐式 TLS on connect(不看端口)。
- **ctx 生效**:`Send` 把底层发送放到 goroutine,`ctx` 取消/超时会让**调用方立即返回**(`ctx.Err()`);后台拨号由 `WithTimeout` 兜底。

---

## 授权(可插拔、对外开放)

```go
mail.BasicAuth(username, password)   // 账号+密码(PLAIN/LOGIN),最常见
mail.NoAuth()                        // 无鉴权(内网中继)
mail.XOAuth2(username, tokenSource)  // OAuth2 Bearer(SASL XOAUTH2),Gmail/Outlook/O365 现代认证
```

`XOAuth2` 的 token 每次发送现取(带 ctx),短时 token 不会过期:

```go
mail.XOAuth2("me@outlook.com", func(ctx context.Context) (string, error) {
    return oauthClient.AccessToken(ctx)   // 你的 token 逻辑
})
```

### 加一种自己的授权方式

授权是**开放**的:实现 `SMTPAuth`(只依赖本接口 + 标准库 `net/smtp`,**不碰底层 SMTP 库**):

```go
type SMTPAuth interface {
    Apply(ctx context.Context, d SMTPDialer) error
}
type SMTPDialer interface {
    SetBasic(username, password string)          // 账号+密码
    SetMechanism(username string, mech smtp.Auth) // 自定义 SASL 机制
}

// 例:自定义机制
type myAuth struct{ user string }
func (a myAuth) Apply(ctx context.Context, d mail.SMTPDialer) error {
    d.SetMechanism(a.user, myMechanism{...}) // myMechanism 实现 net/smtp.Auth
    return nil
}
// 用:mail.NewSMTP(host, port, mail.WithAuth(myAuth{...}))
```

---

## 加一个新供应商

每个供应商就是一个返回 `Mailer` 的构造器。要接阿里云邮件推送 / AWS SES / SendGrid,新增一个实现即可(建议放子包如 `feature/mail/aliyun`、`feature/mail/ses`,让各自的 SDK 依赖不污染核心):

```go
package aliyun
func New(cfg Config) mail.Mailer { /* 实现 Send:调阿里云 DirectMail API */ }
```

调用方只认 `mail.Mailer`,换供应商零改动。

---

## 从旧版迁移(破坏性变更)

旧的 `feature.NewMailFeature()` / `EmailService`(`SendText/SendHTML/Send`)/ `config.MailConfig`(从 `MAIL_SMTP_*` env 读、`bootstrap` 自动注册)**已删除**。迁移:

| 旧 | 新 |
|---|---|
| `app.AddFeature(feature.NewMailFeature())` + 注入 `EmailService` | 直接 `mail.NewSMTP(host, port, opts...)` 构造 `Mailer` |
| 配 `MAIL_SMTP_*` env | 配置来源交给 app(env / DB / 临时),自己读出来传给 `NewSMTP` / `WithAuth` |
| `svc.SendText(ctx, to, subject, body)` | `m.Send(ctx, mail.Message{To: to, Subject: subject, Text: body})` |
| `SendHTML` | `Send(..., Message{HTML: ...})` |
| ctx 不生效 | ctx 生效(可取消/超时) |

`bootstrap.InitDefaultApp` 不再自动注册邮件。
