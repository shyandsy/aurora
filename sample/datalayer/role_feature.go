package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/entity"
	"gorm.io/gorm"
)

type RoleFeatureDatalayer interface {
	GetByID(ctx context.Context, id int64) (*entity.RoleFeature, error)
	GetByRoleID(ctx context.Context, roleID int64) ([]entity.RoleFeature, error)
	GetByRoleIDAndFeatureID(ctx context.Context, roleID, featureID int64) (*entity.RoleFeature, error)
	Create(ctx context.Context, roleFeature *entity.RoleFeature) error
	Delete(ctx context.Context, id int64) error
}

// roleFeatureDatalayer 角色功能关联数据访问层
type roleFeatureDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewRoleFeatureDatalayer 创建角色功能关联数据访问层
func NewRoleFeatureDatalayer(app contracts.App) RoleFeatureDatalayer {
	dl := &roleFeatureDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve RoleFeatureDatalayer: %w", err))
	}
	return dl
}

// GetByID 根据ID获取角色功能关联
func (d *roleFeatureDatalayer) GetByID(ctx context.Context, id int64) (*entity.RoleFeature, error) {
	var roleFeature entity.RoleFeature
	db := d.DB.WithContext(ctx)
	if err := db.First(&roleFeature, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &roleFeature, nil
}

// GetByRoleID 根据角色ID获取角色功能关联列表
func (d *roleFeatureDatalayer) GetByRoleID(ctx context.Context, roleID int64) ([]entity.RoleFeature, error) {
	var roleFeatures []entity.RoleFeature
	db := d.DB.WithContext(ctx)
	if err := db.Where("role_id = ?", roleID).Find(&roleFeatures).Error; err != nil {
		return nil, err
	}
	return roleFeatures, nil
}

// GetByRoleIDAndFeatureID 根据角色ID和功能ID获取角色功能关联
func (d *roleFeatureDatalayer) GetByRoleIDAndFeatureID(ctx context.Context, roleID, featureID int64) (*entity.RoleFeature, error) {
	var roleFeature entity.RoleFeature
	db := d.DB.WithContext(ctx)
	if err := db.Where("role_id = ? AND feature_id = ?", roleID, featureID).First(&roleFeature).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &roleFeature, nil
}

// Create 创建角色功能关联
func (d *roleFeatureDatalayer) Create(ctx context.Context, roleFeature *entity.RoleFeature) error {
	db := d.DB.WithContext(ctx)
	return db.Create(roleFeature).Error
}

// Delete 删除角色功能关联
func (d *roleFeatureDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.RoleFeature{}, id).Error
}
