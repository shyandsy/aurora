package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/entity"
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

// featureDatalayer is the feature data access layer.
type featureDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewFeatureDatalayer creates the feature datalayer.
func NewFeatureDatalayer(app contracts.App) FeatureDatalayer {
	dl := &featureDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve FeatureDatalayer: %w", err))
	}
	return dl
}

// GetByID gets feature by ID.
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

// GetByName gets feature by name.
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

// GetAll gets all features.
func (d *featureDatalayer) GetAll(ctx context.Context) ([]entity.Feature, error) {
	var features []entity.Feature
	db := d.DB.WithContext(ctx)
	if err := db.Find(&features).Error; err != nil {
		return nil, err
	}
	return features, nil
}

// GetByRoleID gets features by role ID.
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

// Create creates a feature.
func (d *featureDatalayer) Create(ctx context.Context, feature *entity.Feature) error {
	db := d.DB.WithContext(ctx)
	return db.Create(feature).Error
}

// Update updates a feature.
func (d *featureDatalayer) Update(ctx context.Context, feature *entity.Feature) error {
	db := d.DB.WithContext(ctx)
	return db.Save(feature).Error
}

// Delete deletes a feature.
func (d *featureDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.Feature{}, id).Error
}
