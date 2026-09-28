package dto

// IssueMicroserviceTokenReq 签发微服务 JWT token 请求
type IssueMicroserviceTokenReq struct {
	// MicroserviceFeatureIDs 微服务功能配置 ID 数组
	MicroserviceFeatureIDs []int64 `json:"microserviceFeatureIds" binding:"required,min=1"`
}

// IssueMicroserviceTokenResp 签发微服务 JWT token 响应
type IssueMicroserviceTokenResp struct {
	// Token JWT token
	Token string `json:"token"`
	// Issuer 签发者
	Issuer string `json:"issuer"`
	// ExpiresAt 过期时间（Unix 时间戳）
	ExpiresAt int64 `json:"expiresAt"`
	// ExpiresIn 过期时间（秒）
	ExpiresIn int64 `json:"expiresIn"`
	// TokenRecordID token 记录 ID
	TokenRecordID int64 `json:"tokenRecordId"`
}

// MicroserviceTokenFeatureToken 微服务 token 记录 DTO
type MicroserviceTokenFeatureToken struct {
	ID                     int64    `json:"id"`
	MicroserviceFeatureIDs []int64  `json:"microserviceFeatureIds"`
	Description            string   `json:"description"`
	FeatureList            []string `json:"featureList"`
	Token                  string   `json:"token"`
	Issuer                 string   `json:"issuer"`
	ExpiresAt              string   `json:"expiresAt"`
	ExpiresIn              int64    `json:"expiresIn"`
	Status                 string   `json:"status"`
	Created                string   `json:"created"`
	Modified               string   `json:"modified"`
}
