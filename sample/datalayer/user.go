package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/entity"
	"gorm.io/gorm"
)

type UserDatalayer interface {
	GetAll(ctx context.Context, offset, limit int) ([]entity.User, int64, error)
	GetByID(ctx context.Context, id int64) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id int64) error
}

// userDatalayer 用户数据访问层
type userDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewUserDatalayer 创建用户数据访问层
func NewUserDatalayer(app contracts.App) UserDatalayer {
	dl := &userDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve UserDatalayer: %w", err))
	}
	return dl
}

// GetAll 获取所有用户（分页）
func (d *userDatalayer) GetAll(ctx context.Context, offset, limit int) ([]entity.User, int64, error) {
	var users []entity.User
	var total int64
	db := d.DB.WithContext(ctx)

	// 获取总数
	if err := db.Model(&entity.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据，预加载 Role
	if err := db.Preload("Role").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetByID 根据ID获取用户
func (d *userDatalayer) GetByID(ctx context.Context, id int64) (*entity.User, error) {
	var user entity.User
	db := d.DB.WithContext(ctx)
	if err := db.Preload("Role").First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByEmail 根据邮箱获取用户
func (d *userDatalayer) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	db := d.DB.WithContext(ctx)
	if err := db.Preload("Role").Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// Create 创建用户
func (d *userDatalayer) Create(ctx context.Context, user *entity.User) error {
	db := d.DB.WithContext(ctx)
	return db.Create(user).Error
}

// Update 更新用户
func (d *userDatalayer) Update(ctx context.Context, user *entity.User) error {
	db := d.DB.WithContext(ctx)
	return db.Save(user).Error
}

// Delete 删除用户
func (d *userDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.User{}, id).Error
}
