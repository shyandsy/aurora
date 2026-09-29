package entity

import "time"

// UserSession 一次登录的会话/设备记录。refresh 续期算同一会话(session_id 跨 refresh 不变)。
// 见 migrations/20260921120000_user_session.sql。撤销 = 标 Revoked + 走 tokenguard.Guard 删该会话
// access/refresh jti 的 IP 绑定(Guard.VerifySession fail-close → 即时失效)。
type UserSession struct {
	ID            int64      `gorm:"primaryKey;column:id" json:"id"`
	SessionID     string     `gorm:"column:session_id;type:varchar(64);not null;uniqueIndex:uk_session_id" json:"sessionId"`
	UserID        int64      `gorm:"column:user_id;not null;index:idx_user_active,priority:1" json:"userId"`
	ClientType    string     `gorm:"column:client_type;type:varchar(16);not null;default:'web'" json:"clientType"`
	DeviceName    string     `gorm:"column:device_name;type:varchar(128);not null;default:''" json:"deviceName"`
	UserAgent     string     `gorm:"column:user_agent;type:varchar(512);not null;default:''" json:"userAgent"`
	LoginIP       string     `gorm:"column:login_ip;type:varchar(64);not null;default:''" json:"loginIp"`
	LastIP        string     `gorm:"column:last_ip;type:varchar(64);not null;default:''" json:"lastIp"`
	LoginGeo      string     `gorm:"column:login_geo;type:varchar(128);not null;default:''" json:"loginGeo"`
	LastGeo       string     `gorm:"column:last_geo;type:varchar(128);not null;default:''" json:"lastGeo"`
	CurAccessJTI  string     `gorm:"column:cur_access_jti;type:varchar(64);not null;default:''" json:"-"`
	CurRefreshJTI string     `gorm:"column:cur_refresh_jti;type:varchar(64);not null;default:'';index:idx_refresh_jti" json:"-"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"createdAt"`
	LastSeenAt    time.Time  `gorm:"column:last_seen_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"lastSeenAt"`
	ExpiresAt     *time.Time `gorm:"column:expires_at" json:"expiresAt"`
	Revoked       bool       `gorm:"column:revoked;type:tinyint(1);not null;default:0;index:idx_user_active,priority:2" json:"revoked"`
	RevokedAt     *time.Time `gorm:"column:revoked_at" json:"revokedAt"`
	RevokedReason string     `gorm:"column:revoked_reason;type:varchar(64);not null;default:''" json:"revokedReason"`
}

// TableName 指定表名。
func (UserSession) TableName() string { return "user_session" }
