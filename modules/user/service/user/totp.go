package user

import (
	"os"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// userTotpIssuerNameEnv 展示在 authenticator app 里的发行方名(品牌走 env,不写死;中性化,可复制)。
const userTotpIssuerNameEnv = "USER_TOTP_ISSUER_NAME"

// defaultTotpIssuer 未配置 USER_TOTP_ISSUER_NAME 时的中性兜底名(不暴露品牌/敏感信息)。
const defaultTotpIssuer = "Account"

// totpIssuer 返回展示用发行方名:优先 env,缺省用中性兜底名。
func totpIssuer() string {
	if v := strings.TrimSpace(os.Getenv(userTotpIssuerNameEnv)); v != "" {
		return v
	}
	return defaultTotpIssuer
}

// generateTotpSecret 生成 TOTP 密钥。**标准参数 SHA1 / 6 位 / 30 秒**(pquerna/otp 默认值)——
// Google Authenticator 只认这套,别自定义算法/位数否则它不认。
// 返回 base32 密钥(供手动录入)+ otpauth:// URI(前端据此渲染二维码)。
func generateTotpSecret(email string) (secret, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer(),
		AccountName: email,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// validateTotp 校验 6 位码;±1 周期(30s)容差,吸收服务端/手机的时钟偏移。
func validateTotp(secret, code string) bool {
	ok, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && ok
}
