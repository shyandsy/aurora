package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/entity"
	"gorm.io/gorm"
)

type RoleFeatureDatalayer interface {
	GetByID(ctx context.Context, id int64) (*entity.RoleFeature, error)
	GetByRoleID(ctx context.Context, roleID int64) ([]entity.RoleFeature, error)
	GetByRoleIDAndFeatureID(ctx context.Context, roleID, featureID int64) (*entity.RoleFeature, error)
	Create(ctx context.Context, roleFeature *entity.RoleFeature) error
	Delete(ctx context.Context, id int64) error
}

// roleFeatureDatalayer is the role-feature data access layer.
type roleFeatureDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewRoleFeatureDatalayer creates the role-feature datalayer.
func NewRoleFeatureDatalayer(app contracts.App) RoleFeatureDatalayer {
	dl := &roleFeatureDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve RoleFeatureDatalayer: %w", err))
	}
	return dl
}

// GetByID gets role-feature by ID.
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

// GetByRoleID gets role-features by role ID.
func (d *roleFeatureDatalayer) GetByRoleID(ctx context.Context, roleID int64) ([]entity.RoleFeature, error) {
	var roleFeatures []entity.RoleFeature
	db := d.DB.WithContext(ctx)
	if err := db.Where("role_id = ?", roleID).Find(&roleFeatures).Error; err != nil {
		return nil, err
	}
	return roleFeatures, nil
}

// GetByRoleIDAndFeatureID gets role-feature by role ID and feature ID.
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

// Create creates a role-feature.
func (d *roleFeatureDatalayer) Create(ctx context.Context, roleFeature *entity.RoleFeature) error {
	db := d.DB.WithContext(ctx)
	return db.Create(roleFeature).Error
}

// Delete deletes a role-feature.
func (d *roleFeatureDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.RoleFeature{}, id).Error
}
