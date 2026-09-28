package datalayer

import (
	"context"
	"fmt"
	"time"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/entity"
	"gorm.io/gorm"
)

// UserSessionDatalayer 登录会话/设备清单(user_session)数据访问层。
type UserSessionDatalayer interface {
	Create(ctx context.Context, s *entity.UserSession) error
	// GetByRefreshJTI 按当前 refresh jti 定位会话(refresh 续期时用旧 jti 找回同一会话)。无则返回 (nil,nil)。
	GetByRefreshJTI(ctx context.Context, refreshJTI string) (*entity.UserSession, error)
	// GetByUserAndSessionID 取某用户名下指定会话(撤销前校验归属)。无则 (nil,nil)。
	GetByUserAndSessionID(ctx context.Context, userID int64, sessionID string) (*entity.UserSession, error)
	// ListActiveByUser 列某用户未撤销且未过期的会话(最近活跃在前)。
	ListActiveByUser(ctx context.Context, userID int64) ([]entity.UserSession, error)
	// Rotate 续期时更新会话的 jti/最近活跃/来源(按会话 id)。
	Rotate(ctx context.Context, id int64, accessJTI, refreshJTI, lastIP, lastGeo string, expiresAt *time.Time) error
	// Revoke 标记撤销(按 user + sessionID,防越权撤别人的);返回被撤销的会话(供上层删 tokenip 绑定)。无则 (nil,nil)。
	Revoke(ctx context.Context, userID int64, sessionID, reason string) (*entity.UserSession, error)
	// RevokeOthers 撤销该用户除 keepSessionID 外的所有活跃会话,返回被撤销的会话列表。
	RevokeOthers(ctx context.Context, userID int64, keepSessionID, reason string) ([]entity.UserSession, error)
}

type userSessionDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewUserSessionDatalayer 创建会话数据访问层。
func NewUserSessionDatalayer(app contracts.App) UserSessionDatalayer {
	dl := &userSessionDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve UserSessionDatalayer: %w", err))
	}
	return dl
}

func (d *userSessionDatalayer) Create(ctx context.Context, s *entity.UserSession) error {
	return d.DB.WithContext(ctx).Create(s).Error
}

func (d *userSessionDatalayer) GetByRefreshJTI(ctx context.Context, refreshJTI string) (*entity.UserSession, error) {
	if refreshJTI == "" {
		return nil, nil
	}
	var s entity.UserSession
	err := d.DB.WithContext(ctx).Where("cur_refresh_jti = ?", refreshJTI).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *userSessionDatalayer) GetByUserAndSessionID(ctx context.Context, userID int64, sessionID string) (*entity.UserSession, error) {
	var s entity.UserSession
	err := d.DB.WithContext(ctx).Where("user_id = ? AND session_id = ?", userID, sessionID).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *userSessionDatalayer) ListActiveByUser(ctx context.Context, userID int64) ([]entity.UserSession, error) {
	var rs []entity.UserSession
	err := d.DB.WithContext(ctx).
		Where("user_id = ? AND revoked = 0 AND (expires_at IS NULL OR expires_at > ?)", userID, time.Now()).
		Order("last_seen_at DESC").Find(&rs).Error
	return rs, err
}

func (d *userSessionDatalayer) Rotate(ctx context.Context, id int64, accessJTI, refreshJTI, lastIP, lastGeo string, expiresAt *time.Time) error {
	return d.DB.WithContext(ctx).Model(&entity.UserSession{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"cur_access_jti":  accessJTI,
			"cur_refresh_jti": refreshJTI,
			"last_ip":         lastIP,
			"last_geo":        lastGeo,
			"last_seen_at":    time.Now(),
			"expires_at":      expiresAt,
		}).Error
}

func (d *userSessionDatalayer) Revoke(ctx context.Context, userID int64, sessionID, reason string) (*entity.UserSession, error) {
	s, err := d.GetByUserAndSessionID(ctx, userID, sessionID)
	if err != nil || s == nil || s.Revoked {
		return nil, err
	}
	now := time.Now()
	if err := d.markRevoked(ctx, s.ID, reason, now); err != nil {
		return nil, err
	}
	return s, nil
}

func (d *userSessionDatalayer) RevokeOthers(ctx context.Context, userID int64, keepSessionID, reason string) ([]entity.UserSession, error) {
	var rs []entity.UserSession
	if err := d.DB.WithContext(ctx).
		Where("user_id = ? AND revoked = 0 AND session_id <> ?", userID, keepSessionID).
		Find(&rs).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range rs {
		if err := d.markRevoked(ctx, rs[i].ID, reason, now); err != nil {
			return nil, err
		}
	}
	return rs, nil
}

func (d *userSessionDatalayer) markRevoked(ctx context.Context, id int64, reason string, at time.Time) error {
	return d.DB.WithContext(ctx).Model(&entity.UserSession{}).Where("id = ?", id).
		Updates(map[string]interface{}{"revoked": true, "revoked_at": at, "revoked_reason": reason}).Error
}
