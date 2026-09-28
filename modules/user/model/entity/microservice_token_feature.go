package entity

import (
	"time"

	"github.com/shyandsy/aurora/types"
)

// MicroserviceTokenFeature 微服务 token 功能配置
type MicroserviceTokenFeature struct {
	ID          int64                   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string                  `gorm:"column:name;type:varchar(255);not null;uniqueIndex:uk_name" json:"name"`
	Description string                  `gorm:"column:description;type:varchar(500);not null" json:"description"`
	FeatureList types.JSONArray[string] `gorm:"column:feature_list;type:json;not null" json:"featureList"`
	Created     time.Time               `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created"`
	Modified    time.Time               `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"modified"`
}

// TableName 指定表名
func (MicroserviceTokenFeature) TableName() string {
	return "user_microservice_token_features"
}

// MicroserviceTokenFeatureToken 生成的微服务 token 记录
type MicroserviceTokenFeatureToken struct {
	ID                     int64                   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	MicroserviceFeatureIDs types.JSONArray[int64]  `gorm:"column:microservice_feature_ids;type:json;not null" json:"microserviceFeatureIds"`
	Description            string                  `gorm:"column:description;type:text" json:"description"`
	FeatureList            types.JSONArray[string] `gorm:"column:feature_list;type:json;not null" json:"featureList"`
	Token                  string                  `gorm:"column:token;type:text;not null" json:"token"`
	Issuer                 string                  `gorm:"column:issuer;type:varchar(255);not null" json:"issuer"`
	ExpiresAt              time.Time               `gorm:"column:expires_at;type:datetime;not null" json:"expiresAt"`
	ExpiresIn              int64                   `gorm:"column:expires_in;type:bigint;not null" json:"expiresIn"`
	Status                 string                  `gorm:"column:status;type:varchar(20);not null;default:'ENABLED'" json:"status"`
	Created                time.Time               `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created"`
	Modified               time.Time               `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"modified"`
}

// TableName 指定表名
func (MicroserviceTokenFeatureToken) TableName() string {
	return "user_microservice_token_features_token"
}
