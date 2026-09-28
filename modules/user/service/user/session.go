package user

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/logger"

	"github.com/shyandsy/aurora/feature/geoip"

	"github.com/shyandsy/aurora/middleware"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	"github.com/shyandsy/aurora/modules/user/model/entity"
)

// clientTypeWeb 是 recordSession 兜底用的默认 client_type(实际值由签发时的作用域名给出)。
const clientTypeWeb = "web"

// recordSession 登录成功后记一条会话(best-effort:失败只记日志,绝不阻断登录)。
// 只给「正式 token」记录;2FA 仅绑定 token 是受限过渡态,不记会话。clientType = 登录作用域名(web/app/……)。
func (s *userService) recordSession(ctx *contracts.RequestContext, userID int64, tok *auroraFeature.TokenResponse, clientType string) {
	if tok == nil {
		return
	}
	if clientType == "" {
		clientType = clientTypeWeb
	}
	ac, aerr := s.JWT.ValidateToken(tok.AccessToken)
	rc, rerr := s.JWT.ValidateRefreshToken(tok.RefreshToken)
	if aerr != nil || rerr != nil {
		logger.Errorf("recordSession: 解析新 token 失败(跳过会话记录): access=%v refresh=%v", aerr, rerr)
		return
	}
	sid, err := newSessionID()
	if err != nil {
		logger.Errorf("recordSession: 生成 session_id 失败: %v", err)
		return
	}
	ip := ctx.ClientIP()
	geo := geoDisplay(s.Geo.Lookup(ip))
	ua := ctx.GetHeader("User-Agent")
	sess := &entity.UserSession{
		SessionID:     sid,
		UserID:        userID,
		ClientType:    clientType,
		DeviceName:    deviceNameFromUA(ua),
		UserAgent:     truncate(ua, 512),
		LoginIP:       ip,
		LastIP:        ip,
		LoginGeo:      geo,
		LastGeo:       geo,
		CurAccessJTI:  ac.ID,
		CurRefreshJTI: rc.ID,
		ExpiresAt:     claimsExpiry(rc),
	}
	if err := s.SessionDL.Create(ctx.Context, sess); err != nil {
		logger.Errorf("recordSession: 写会话失败: %v", err)
	}
}

// rotateSession refresh 续期时把会话滚动到新 jti(按旧 refresh jti 定位)。best-effort。
func (s *userService) rotateSession(ctx *contracts.RequestContext, oldRefreshJTI string, tok *auroraFeature.TokenResponse) {
	if tok == nil || oldRefreshJTI == "" {
		return
	}
	sess, err := s.SessionDL.GetByRefreshJTI(ctx.Context, oldRefreshJTI)
	if err != nil || sess == nil {
		return // 找不到(旧 token 早于本功能上线等)→ 静默跳过
	}
	ac, aerr := s.JWT.ValidateToken(tok.AccessToken)
	rc, rerr := s.JWT.ValidateRefreshToken(tok.RefreshToken)
	if aerr != nil || rerr != nil {
		return
	}
	ip := ctx.ClientIP()
	if err := s.SessionDL.Rotate(ctx.Context, sess.ID, ac.ID, rc.ID, ip, geoDisplay(s.Geo.Lookup(ip)), claimsExpiry(rc)); err != nil {
		logger.Errorf("rotateSession: 更新会话失败: %v", err)
		return
	}
	// 轮换即失效旧的一对(此刻 sess 仍持旧 jti):删 IP 绑定 + 入 jti 黑名单。
	// aurora 的 RefreshToken 不拉黑旧 refresh(旧 refresh 24h 内仍可续期),不失效旧的话:
	// ①旧 refresh 可被重放续期;②撤销只覆盖"当前",链条里更早的活 token 撤不掉。
	// 失效后任一时刻只有"当前"这对有效 → 撤销(删当前)即对整条链完整。客户端已切到新 token,无碍。
	// App token 无 IP 绑定,靠 jti 黑名单那半生效(Guard.Revoke 里删 IP 绑定那半对它是无害 no-op)。
	s.killSessionTokens(ctx, sess)
}

// revokeSessionByRefreshToken 登出时把对应会话标为已撤销(best-effort;须在 aurora Logout 拉黑前调,
// 否则 refresh 已进黑名单、ValidateRefreshToken 会失败拿不到 jti)。
func (s *userService) revokeSessionByRefreshToken(ctx *contracts.RequestContext, refreshToken string) {
	if refreshToken == "" {
		return
	}
	rc, err := s.JWT.ValidateRefreshToken(refreshToken)
	if err != nil {
		return
	}
	sess, err := s.SessionDL.GetByRefreshJTI(ctx.Context, rc.ID)
	if err != nil || sess == nil {
		return
	}
	if _, err := s.SessionDL.Revoke(ctx.Context, sess.UserID, sess.SessionID, "logout"); err != nil {
		logger.Errorf("revokeSessionByRefreshToken: 撤销会话失败: %v", err)
	}
}

// ListSessions 列出当前用户的活跃会话(标出"当前这台")。
func (s *userService) ListSessions(ctx *contracts.RequestContext) ([]dto.SessionView, bizerr.BizError) {
	userID, ok := middleware.GetUserID(ctx.Context)
	if !ok {
		return nil, bizerr.New(401, errors.New(ctx.T("auth.unauthorized")))
	}
	rows, err := s.SessionDL.ListActiveByUser(ctx.Context, userID)
	if err != nil {
		return nil, internalErr(ctx, "user", err)
	}
	curJTI := s.currentAccessJTI(ctx)
	out := make([]dto.SessionView, 0, len(rows))
	for i := range rows {
		out = append(out, dto.SessionView{
			SessionID:  rows[i].SessionID,
			ClientType: rows[i].ClientType,
			DeviceName: rows[i].DeviceName,
			LoginIP:    rows[i].LoginIP,
			LastIP:     rows[i].LastIP,
			LoginGeo:   rows[i].LoginGeo,
			LastGeo:    rows[i].LastGeo,
			CreatedAt:  rows[i].CreatedAt,
			LastSeenAt: rows[i].LastSeenAt,
			Current:    curJTI != "" && rows[i].CurAccessJTI == curJTI,
		})
	}
	return out, nil
}

// RevokeSession 撤销指定会话(仅能撤自己名下的):标 revoked + 删 tokenip 绑定(即时失效)。
func (s *userService) RevokeSession(ctx *contracts.RequestContext, sessionID string) bizerr.BizError {
	userID, ok := middleware.GetUserID(ctx.Context)
	if !ok {
		return bizerr.New(401, errors.New(ctx.T("auth.unauthorized")))
	}
	sess, err := s.SessionDL.Revoke(ctx.Context, userID, sessionID, "user")
	if err != nil {
		return internalErr(ctx, "user", err)
	}
	if sess != nil {
		s.killSessionTokens(ctx, sess)
	}
	return nil
}

// RevokeOtherSessions 撤销当前用户除"当前这台"外的所有活跃会话(一键"登出所有其他设备")。
func (s *userService) RevokeOtherSessions(ctx *contracts.RequestContext) bizerr.BizError {
	userID, ok := middleware.GetUserID(ctx.Context)
	if !ok {
		return bizerr.New(401, errors.New(ctx.T("auth.unauthorized")))
	}
	// 找出"当前这台"的 sessionID(据当前 access jti),避免把自己也踢了。
	keep := ""
	if curJTI := s.currentAccessJTI(ctx); curJTI != "" {
		if rows, err := s.SessionDL.ListActiveByUser(ctx.Context, userID); err == nil {
			for i := range rows {
				if rows[i].CurAccessJTI == curJTI {
					keep = rows[i].SessionID
					break
				}
			}
		}
	}
	// 定位不到"当前会话"(currentAccessJTI 解析失败/会话没记上)→ 中止,绝不 keep="" 把自己也一起撤了。
	if keep == "" {
		return bizerr.ErrInternalServerError(errors.New("无法确定当前会话,已中止(避免误撤当前设备)"))
	}
	revoked, err := s.SessionDL.RevokeOthers(ctx.Context, userID, keep, "user")
	if err != nil {
		return internalErr(ctx, "user", err)
	}
	for i := range revoked {
		s.killSessionTokens(ctx, &revoked[i])
	}
	return nil
}

// killSessionTokens 即时失效会话的当前 access/refresh 一对。「彻底吊销一枚 token」= 删 IP 绑定 + 入 jti
// 黑名单,这一领域知识收口在 tokenguard.Revoke(web 靠删绑定即时失效、app 无绑定靠黑名单、覆盖 Proxied 面)。
func (s *userService) killSessionTokens(ctx *contracts.RequestContext, sess *entity.UserSession) {
	// 会话表只存了 jti(无 claims),故走 Guard.Revoke(denyTTL 兜底)。best-effort:失败只记日志、不硬失败。
	if err := s.TokenGuard.Revoke(ctx.Context, sess.CurAccessJTI); err != nil {
		logger.Errorf("killSessionTokens: 撤销 access token 失败(best-effort): %v", err)
	}
	if err := s.TokenGuard.Revoke(ctx.Context, sess.CurRefreshJTI); err != nil {
		logger.Errorf("killSessionTokens: 撤销 refresh token 失败(best-effort): %v", err)
	}
}

// currentAccessJTI 从本次请求的 Authorization access token 解出 jti(用于标记/保留"当前会话")。取不到返回 ""。
func (s *userService) currentAccessJTI(ctx *contracts.RequestContext) string {
	tok := extractBearerToken(ctx.GetHeader("Authorization"))
	if tok == "" {
		return ""
	}
	if c, err := s.JWT.ValidateToken(tok); err == nil {
		return c.ID
	}
	return ""
}

// --- 小工具 ---

func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// geoDisplay 把 aurora geoip.Record 压成去空段、以「·」连接的可读归属地串(如「CN·广东省·深圳市·电信」)。
// 空/未知字段跳过;整体查不到返回 ""。会话清单据此展示登录地点。
func geoDisplay(rec geoip.Record) string {
	parts := make([]string, 0, 4)
	for _, p := range []string{rec.CountryISO, rec.Province, rec.City, rec.ISP} {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "·")
}

func claimsExpiry(c *auroraFeature.Claims) *time.Time {
	if c == nil || c.ExpiresAt == nil {
		return nil
	}
	t := c.ExpiresAt.Time
	return &t
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// deviceNameFromUA 从 User-Agent 粗提"浏览器 · 系统"(纯启发式,无第三方库)。取不到返回 "未知设备"。
func deviceNameFromUA(ua string) string {
	if ua == "" {
		return "未知设备"
	}
	browser := firstMatch(ua, []kv{
		{"Edg", "Edge"}, {"OPR", "Opera"}, {"Firefox", "Firefox"},
		{"Chrome", "Chrome"}, {"Safari", "Safari"},
	})
	os := firstMatch(ua, []kv{
		{"iPhone", "iPhone"}, {"iPad", "iPad"}, {"Android", "Android"},
		{"Windows", "Windows"}, {"Mac OS", "macOS"}, {"Macintosh", "macOS"}, {"Linux", "Linux"},
	})
	switch {
	case browser != "" && os != "":
		return browser + " · " + os
	case os != "":
		return os
	case browser != "":
		return browser
	default:
		return "未知设备"
	}
}

type kv struct{ needle, label string }

func firstMatch(hay string, pairs []kv) string {
	for _, p := range pairs {
		if strings.Contains(hay, p.needle) {
			return p.label
		}
	}
	return ""
}
