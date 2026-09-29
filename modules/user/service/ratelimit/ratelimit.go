// Package ratelimit 是 user 服务「被锁登录」的后台管理服务层:列出当前被 loginguard 锁住的 IP/账号、
// 手动解锁。数据在 loginguard 的引擎里(rate_limit:user:login_*);loginguard 引擎无 SCAN,故登录失败时
// 往一个 user 专属索引 hash 记一笔(IP/账号 → 最近邮箱 + 时间),这里 HGetAll 拿候选,再逐个用
// LoginGuard 预检复核「是否仍被锁」,过期项顺手 HDel 剔除。解锁走 LoginGuard.Unlock(清失败计数 + 锁)。
//
// 只管**登录**这一类(user 服务只有登录限流)。与 customer 的「限流客户」台按 namespace 各读各的、互不相扰。
package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/feature/loginguard"
	"github.com/shyandsy/aurora/logger"

	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// 索引 hash 的 key 与 loginguard 引擎**同 namespace**:引擎的锁/计数是 rate_limit:<ns>:login_*,
// 索引必须落在同一个 <ns> 下,否则后台列表/解锁会和真正的锁脱钩(namespace≠"user" 时静默坏)。
// 与 customer 等其它服务按各自 namespace 相互隔离。namespace 由模块 Config.RateLimitNamespace 提供,
// 经构造器/参数一路传进来(不写死、不走包级全局)。
func indexKeyIP(ns string) string      { return "rate_limit:" + ns + ":index" }         // field=IP → 最近尝试
func indexKeyAccount(ns string) string { return "rate_limit:" + ns + ":index:account" } // field=明文 email → 最近尝试

// indexEntry 索引里存的一条「最近尝试」:邮箱(供后台辨认)+ 记录时刻(列表倒序)。
type indexEntry struct {
	Email     string `json:"email"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (e indexEntry) marshal() string { b, _ := json.Marshal(e); return string(b) }

func parseIndexEntry(raw string) indexEntry {
	var e indexEntry
	_ = json.Unmarshal([]byte(raw), &e)
	return e
}

// RecordFailureIndex 登录失败时把 IP(及明文 email 账号)记进 namespace 专属索引,供后台「被锁列表」枚举。
// best-effort:失败只记日志,绝不影响登录主流程。由 user 服务的 loginguard 接入层在记失败时调用。
// ns 必须与 loginguard 引擎、后台 LockAdminService 用的是同一个(模块 Config.RateLimitNamespace)。
func RecordFailureIndex(ctx context.Context, redis auroraFeature.RedisService, ns, ip, email string) {
	if redis == nil {
		return
	}
	entry := indexEntry{Email: email, UpdatedAt: time.Now().Unix()}.marshal()
	if ip != "" {
		if err := redis.HSet(ctx, indexKeyIP(ns), ip, entry); err != nil {
			logger.Errorf("user ratelimit: 写 IP 索引失败(best-effort): %v", err)
		}
	}
	if email != "" {
		if err := redis.HSet(ctx, indexKeyAccount(ns), email, entry); err != nil {
			logger.Errorf("user ratelimit: 写账号索引失败(best-effort): %v", err)
		}
	}
}

// LockAdminService user 登录「被锁列表 + 解锁」后台服务。
type LockAdminService interface {
	// ListLocked 列出当前仍被锁的 IP + 账号(带最近邮箱、命中原因、解除剩余秒数);读时剔除已自动恢复的过期项。
	ListLocked(ctx *contracts.RequestContext) (*dto.UserLockedListResp, bizerr.BizError)
	// Unlock 按 scope+key 强制解锁(IP 或账号),立即恢复。
	Unlock(ctx *contracts.RequestContext, scope, key string) bizerr.BizError
}

type lockAdminService struct {
	LoginGuard loginguard.Guard           `inject:""`
	Redis      auroraFeature.RedisService `inject:""`
}

// NewLockAdminService 创建服务。namespace 不再传入:索引 key 从注入的 LoginGuard.Namespace()
// (= 引擎 namespace = SERVICE_NAME)取,和真正的锁永远同前缀、单一源。
func NewLockAdminService(app contracts.App) LockAdminService {
	s := &lockAdminService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("resolve LockAdminService: %w", err))
	}
	return s
}

// ns 从 Guard 取(引擎的 namespace);集中一处,方法里都用它拼索引 key。
func (s *lockAdminService) ns() string { return s.LoginGuard.Namespace() }

func (s *lockAdminService) ListLocked(ctx *contracts.RequestContext) (*dto.UserLockedListResp, bizerr.BizError) {
	c := ctx.Context
	items := make([]dto.UserLockedEntry, 0)

	// —— IP 维度 ——
	ipIndex, err := s.Redis.HGetAll(c, indexKeyIP(s.ns()))
	if err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}
	var staleIP []string
	for ip, raw := range ipIndex {
		d := s.LoginGuard.PrecheckIP(c, ip)
		if !d.Blocked {
			staleIP = append(staleIP, ip) // 已自动恢复,索引陈迹
			continue
		}
		entry := parseIndexEntry(raw)
		items = append(items, dto.UserLockedEntry{
			Scope:             dto.UserLockScopeIP,
			Key:               ip,
			Email:             entry.Email,
			Reason:            d.Reason,
			RetryAfterSeconds: d.RetryAfter,
			UpdatedAt:         entry.UpdatedAt,
		})
	}
	if len(staleIP) > 0 {
		if _, derr := s.Redis.HDel(c, indexKeyIP(s.ns()), staleIP...); derr != nil {
			logger.Errorf("user ratelimit: 剔除过期 IP 索引失败: %v", derr)
		}
	}

	// —— 账号维度(明文 email)——
	acctIndex, err := s.Redis.HGetAll(c, indexKeyAccount(s.ns()))
	if err != nil {
		logger.Errorf("user ratelimit: 读账号索引失败: %v", err) // 不阻断 IP 结果
	} else {
		var staleAcct []string
		for email, raw := range acctIndex {
			d := s.LoginGuard.PrecheckAccount(c, email)
			if !d.Blocked {
				staleAcct = append(staleAcct, email)
				continue
			}
			entry := parseIndexEntry(raw)
			items = append(items, dto.UserLockedEntry{
				Scope:             dto.UserLockScopeAccount,
				Key:               email,
				Email:             email,
				Reason:            d.Reason,
				RetryAfterSeconds: d.RetryAfter,
				UpdatedAt:         entry.UpdatedAt,
			})
		}
		if len(staleAcct) > 0 {
			if _, derr := s.Redis.HDel(c, indexKeyAccount(s.ns()), staleAcct...); derr != nil {
				logger.Errorf("user ratelimit: 剔除过期账号索引失败: %v", derr)
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt > items[j].UpdatedAt }) // 最近在前
	return &dto.UserLockedListResp{Items: items}, nil
}

func (s *lockAdminService) Unlock(ctx *contracts.RequestContext, scope, key string) bizerr.BizError {
	key = strings.TrimSpace(key)
	if key == "" {
		return bizerr.NewValidationError(ctx.T("error.bad_request"), nil)
	}
	c := ctx.Context
	switch scope {
	case dto.UserLockScopeIP:
		s.LoginGuard.Unlock(c, key, "") // 清 IP 失败计数 + 锁
		if _, err := s.Redis.HDel(c, indexKeyIP(s.ns()), key); err != nil {
			logger.Errorf("user ratelimit: HDel IP 索引 %s 失败: %v", key, err)
		}
	case dto.UserLockScopeAccount:
		email := strings.ToLower(key)
		s.LoginGuard.Unlock(c, "", email) // 清账号失败计数 + 锁
		if _, err := s.Redis.HDel(c, indexKeyAccount(s.ns()), email); err != nil {
			logger.Errorf("user ratelimit: HDel 账号索引 %s 失败: %v", email, err)
		}
	default:
		return bizerr.NewValidationError(ctx.T("error.bad_request"), nil)
	}
	return nil
}
