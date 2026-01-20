package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/entity"
	"gorm.io/gorm"
)

type RoleDatalayer interface {
	GetByID(ctx context.Context, id int64) (*entity.Role, error)
	GetByName(ctx context.Context, name string) (*entity.Role, error)
	GetAll(ctx context.Context) ([]entity.Role, error)
	Create(ctx context.Context, role *entity.Role) error
	Update(ctx context.Context, role *entity.Role) error
	Delete(ctx context.Context, id int64) error
}

// roleDatalayer 角色数据访问层
type roleDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewRoleDatalayer 创建角色数据访问层
func NewRoleDatalayer(app contracts.App) RoleDatalayer {
	dl := &roleDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve RoleDatalayer: %w", err))
	}
	return dl
}

// GetByID 根据ID获取角色
func (d *roleDatalayer) GetByID(ctx context.Context, id int64) (*entity.Role, error) {
	var role entity.Role
	db := d.DB.WithContext(ctx)
	if err := db.First(&role, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &role, nil
}

// GetByName 根据名称获取角色
func (d *roleDatalayer) GetByName(ctx context.Context, name string) (*entity.Role, error) {
	var role entity.Role
	db := d.DB.WithContext(ctx)
	if err := db.Where("name = ?", name).First(&role).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &role, nil
}

// GetAll 获取所有角色
func (d *roleDatalayer) GetAll(ctx context.Context) ([]entity.Role, error) {
	var roles []entity.Role
	db := d.DB.WithContext(ctx)
	if err := db.Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// Create 创建角色
func (d *roleDatalayer) Create(ctx context.Context, role *entity.Role) error {
	db := d.DB.WithContext(ctx)
	return db.Create(role).Error
}

// Update 更新角色
func (d *roleDatalayer) Update(ctx context.Context, role *entity.Role) error {
	db := d.DB.WithContext(ctx)
	return db.Save(role).Error
}

// Delete 删除角色
func (d *roleDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.Role{}, id).Error
}
