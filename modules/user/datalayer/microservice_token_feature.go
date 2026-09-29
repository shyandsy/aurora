package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/entity"
	"gorm.io/gorm"
)

// MicroserviceTokenFeatureDatalayer 微服务 token 功能配置数据层接口
type MicroserviceTokenFeatureDatalayer interface {
	// GetByID 根据 ID 获取配置
	GetByID(ctx context.Context, id int64) (*entity.MicroserviceTokenFeature, error)
	// GetByIDs 根据 ID 列表批量获取配置
	GetByIDs(ctx context.Context, ids []int64) ([]*entity.MicroserviceTokenFeature, error)
	// GetAll 获取所有配置
	GetAll(ctx context.Context) ([]*entity.MicroserviceTokenFeature, error)
	// Create 创建配置
	Create(ctx context.Context, feature *entity.MicroserviceTokenFeature) error
	// Update 更新配置
	Update(ctx context.Context, id int64, feature *entity.MicroserviceTokenFeature) error
	// Delete 删除配置
	Delete(ctx context.Context, id int64) error
}

// microserviceTokenFeatureDatalayer 微服务 token 功能配置数据层实现
type microserviceTokenFeatureDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewMicroserviceTokenFeatureDatalayer 创建微服务 token 功能配置数据层
func NewMicroserviceTokenFeatureDatalayer(app contracts.App) MicroserviceTokenFeatureDatalayer {
	dl := &microserviceTokenFeatureDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve MicroserviceTokenFeatureDatalayer: %w", err))
	}
	return dl
}

// GetByID 根据 ID 获取配置
func (dl *microserviceTokenFeatureDatalayer) GetByID(ctx context.Context, id int64) (*entity.MicroserviceTokenFeature, error) {
	var feature entity.MicroserviceTokenFeature
	if err := dl.DB.WithContext(ctx).Where("id = ?", id).First(&feature).Error; err != nil {
		return nil, err
	}
	return &feature, nil
}

// GetByIDs 根据 ID 列表批量获取配置
func (dl *microserviceTokenFeatureDatalayer) GetByIDs(ctx context.Context, ids []int64) ([]*entity.MicroserviceTokenFeature, error) {
	if len(ids) == 0 {
		return []*entity.MicroserviceTokenFeature{}, nil
	}
	var features []*entity.MicroserviceTokenFeature
	if err := dl.DB.WithContext(ctx).Where("id IN ?", ids).Find(&features).Error; err != nil {
		return nil, err
	}
	return features, nil
}

// GetAll 获取所有配置
func (dl *microserviceTokenFeatureDatalayer) GetAll(ctx context.Context) ([]*entity.MicroserviceTokenFeature, error) {
	var features []*entity.MicroserviceTokenFeature
	if err := dl.DB.WithContext(ctx).Order("created DESC").Find(&features).Error; err != nil {
		return nil, err
	}
	return features, nil
}

// Create 创建配置
func (dl *microserviceTokenFeatureDatalayer) Create(ctx context.Context, feature *entity.MicroserviceTokenFeature) error {
	return dl.DB.WithContext(ctx).Create(feature).Error
}

// Update 更新配置
func (dl *microserviceTokenFeatureDatalayer) Update(ctx context.Context, id int64, feature *entity.MicroserviceTokenFeature) error {
	return dl.DB.WithContext(ctx).Model(&entity.MicroserviceTokenFeature{}).Where("id = ?", id).Updates(feature).Error
}

// Delete 删除配置
func (dl *microserviceTokenFeatureDatalayer) Delete(ctx context.Context, id int64) error {
	return dl.DB.WithContext(ctx).Delete(&entity.MicroserviceTokenFeature{}, id).Error
}

// MicroserviceTokenFeatureTokenDatalayer 生成的微服务 token 记录数据层接口
type MicroserviceTokenFeatureTokenDatalayer interface {
	// Create 创建 token 记录
	Create(ctx context.Context, token *entity.MicroserviceTokenFeatureToken) error
	// GetByID 根据 ID 获取 token 记录
	GetByID(ctx context.Context, id int64) (*entity.MicroserviceTokenFeatureToken, error)
	// GetAll 分页获取 token 记录
	GetAll(ctx context.Context, offset, limit int) ([]*entity.MicroserviceTokenFeatureToken, int64, error)
	// UpdateStatus 更新 token 状态
	UpdateStatus(ctx context.Context, id int64, status string) error
}

// microserviceTokenFeatureTokenDatalayer 生成的微服务 token 记录数据层实现
type microserviceTokenFeatureTokenDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewMicroserviceTokenFeatureTokenDatalayer 创建生成的微服务 token 记录数据层
func NewMicroserviceTokenFeatureTokenDatalayer(app contracts.App) MicroserviceTokenFeatureTokenDatalayer {
	dl := &microserviceTokenFeatureTokenDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve MicroserviceTokenFeatureTokenDatalayer: %w", err))
	}
	return dl
}

// Create 创建 token 记录
func (dl *microserviceTokenFeatureTokenDatalayer) Create(ctx context.Context, token *entity.MicroserviceTokenFeatureToken) error {
	return dl.DB.WithContext(ctx).Create(token).Error
}

// GetByID 根据 ID 获取 token 记录
func (dl *microserviceTokenFeatureTokenDatalayer) GetByID(ctx context.Context, id int64) (*entity.MicroserviceTokenFeatureToken, error) {
	var token entity.MicroserviceTokenFeatureToken
	if err := dl.DB.WithContext(ctx).Where("id = ?", id).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// GetAll 分页获取 token 记录
func (dl *microserviceTokenFeatureTokenDatalayer) GetAll(ctx context.Context, offset, limit int) ([]*entity.MicroserviceTokenFeatureToken, int64, error) {
	var tokens []*entity.MicroserviceTokenFeatureToken
	var total int64
	db := dl.DB.WithContext(ctx)

	// 获取总数
	if err := db.Model(&entity.MicroserviceTokenFeatureToken{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	if err := db.Order("created DESC").Offset(offset).Limit(limit).Find(&tokens).Error; err != nil {
		return nil, 0, err
	}

	return tokens, total, nil
}

// UpdateStatus 更新 token 状态
func (dl *microserviceTokenFeatureTokenDatalayer) UpdateStatus(ctx context.Context, id int64, status string) error {
	return dl.DB.WithContext(ctx).Model(&entity.MicroserviceTokenFeatureToken{}).
		Where("id = ?", id).
		Update("status", status).Error
}
