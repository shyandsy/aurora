package dto

import "time"

// SessionView 「我的登录设备」清单里的一条会话。
type SessionView struct {
	SessionID  string    `json:"sessionId"`
	ClientType string    `json:"clientType"` // web / app
	DeviceName string    `json:"deviceName"`
	LoginIP    string    `json:"loginIp"`
	LastIP     string    `json:"lastIp"`
	LoginGeo   string    `json:"loginGeo"`
	LastGeo    string    `json:"lastGeo"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	Current    bool      `json:"current"` // 是否当前这台(发起本次请求的会话)
}

// RevokeSessionReq 撤销指定会话。
type RevokeSessionReq struct {
	SessionID string `json:"sessionId" binding:"required"`
}
