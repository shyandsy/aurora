package dto

// MicroserviceTokenFeature 微服务 token 功能配置 DTO
type MicroserviceTokenFeature struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	FeatureList []string `json:"featureList"`
	Created     string   `json:"created"`
	Modified    string   `json:"modified"`
}

// CreateMicroserviceTokenFeatureReq 创建微服务 token 功能配置请求
type CreateMicroserviceTokenFeatureReq struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description" binding:"required"`
	FeatureList []string `json:"featureList" binding:"required,min=1"`
}

// UpdateMicroserviceTokenFeatureReq 更新微服务 token 功能配置请求
type UpdateMicroserviceTokenFeatureReq struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	FeatureList []string `json:"featureList,omitempty"`
}
