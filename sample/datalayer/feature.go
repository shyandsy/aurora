package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/entity"
	"gorm.io/gorm"
)

type FeatureDatalayer interface {
	GetByID(ctx context.Context, id int64) (*entity.Feature, error)
	GetByName(ctx context.Context, name string) (*entity.Feature, error)
	GetAll(ctx context.Context) ([]entity.Feature, error)
	GetByRoleID(ctx context.Context, roleID int64) ([]entity.Feature, error)
	Create(ctx context.Context, feature *entity.Feature) error
	Update(ctx context.Context, feature *entity.Feature) error
	Delete(ctx context.Context, id int64) error
}

// featureDatalayer 功能数据访问层
type featureDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewFeatureDatalayer 创建功能数据访问层
func NewFeatureDatalayer(app contracts.App) FeatureDatalayer {
	dl := &featureDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve FeatureDatalayer: %w", err))
	}
	return dl
}

// GetByID 根据ID获取功能
func (d *featureDatalayer) GetByID(ctx context.Context, id int64) (*entity.Feature, error) {
	var feature entity.Feature
	db := d.DB.WithContext(ctx)
	if err := db.First(&feature, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &feature, nil
}

// GetByName 根据名称获取功能
func (d *featureDatalayer) GetByName(ctx context.Context, name string) (*entity.Feature, error) {
	var feature entity.Feature
	db := d.DB.WithContext(ctx)
	if err := db.Where("name = ?", name).First(&feature).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &feature, nil
}

// GetAll 获取所有功能
func (d *featureDatalayer) GetAll(ctx context.Context) ([]entity.Feature, error) {
	var features []entity.Feature
	db := d.DB.WithContext(ctx)
	if err := db.Find(&features).Error; err != nil {
		return nil, err
	}
	return features, nil
}

// GetByRoleID 根据角色ID获取功能列表
func (d *featureDatalayer) GetByRoleID(ctx context.Context, roleID int64) ([]entity.Feature, error) {
	var features []entity.Feature
	db := d.DB.WithContext(ctx)
	if err := db.Table("features").
		Joins("INNER JOIN role_features ON features.id = role_features.feature_id").
		Where("role_features.role_id = ?", roleID).
		Find(&features).Error; err != nil {
		return nil, err
	}
	return features, nil
}

// Create 创建功能
func (d *featureDatalayer) Create(ctx context.Context, feature *entity.Feature) error {
	db := d.DB.WithContext(ctx)
	return db.Create(feature).Error
}

// Update 更新功能
func (d *featureDatalayer) Update(ctx context.Context, feature *entity.Feature) error {
	db := d.DB.WithContext(ctx)
	return db.Save(feature).Error
}

// Delete 删除功能
func (d *featureDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.Feature{}, id).Error
}
