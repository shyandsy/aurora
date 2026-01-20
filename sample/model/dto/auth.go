package dto

// LoginReq 登录请求
type LoginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResp 登录响应
type LoginResp struct {
	AccessToken     string   `json:"accessToken"`
	TokenType       string   `json:"tokenType"`
	ExpiresInSecond int64    `json:"expiresInSecond"`
	RefreshToken    string   `json:"refreshToken"`
	Features        []string `json:"features"`
	User            *User    `json:"user"`
}
