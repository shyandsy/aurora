package user

import (
	"errors"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/logger"
	"golang.org/x/crypto/bcrypt"
)

// internalErr 把原始错误**只记到服务端日志**,向客户端一律返回通用「内部错误」文案。
//
// 绝不把原始 err(可能含 DB/Redis 连接串、内网 host、表/列名、SQL 结构)塞进响应 ——
// 尤其登录/2FA/刷新这类**未认证可达**的路径,否则等于给未登录的人做信息侦察。
// 需要排障就看服务端日志。
func internalErr(ctx *contracts.RequestContext, where string, err error) bizerr.BizError {
	logger.Errorf("[user] internal error at %s: %v", where, err)
	return bizerr.ErrInternalServerError(errors.New(ctx.T("error.internal_server")))
}

// loginFailed 统一的登录失败响应:不存在 / 密码错 / 禁用 一律同一文案、不分字段,防账号枚举。
func loginFailed(ctx *contracts.RequestContext) bizerr.BizError {
	return bizerr.NewValidationError(ctx.T("auth.login_failed"), nil)
}

// dummyBcryptHash:登录时「用户不存在」也跑一次等价 bcrypt,抹平「有效邮箱因跑 bcrypt 更慢」的计时侧信道。
// 启动时算一次(与真实密码同 cost),内容随意 —— 只为消耗等量 CPU 时间。
var dummyBcryptHash []byte

func init() {
	dummyBcryptHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalizer-not-a-real-secret"), bcrypt.DefaultCost)
}
